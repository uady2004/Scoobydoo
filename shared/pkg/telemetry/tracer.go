// Package telemetry configures the OpenTelemetry SDK and exports traces to a
// Jaeger-compatible OTLP/gRPC endpoint for the TikTok-clone platform.
//
// Typical usage:
//
//	tp, err := telemetry.NewTracerProvider(ctx, telemetry.Config{
//	    ServiceName:    "user-service",
//	    ServiceVersion: "v1.2.3",
//	    Environment:    "production",
//	    OTLPEndpoint:   "jaeger:4317",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tp.Shutdown(ctx)
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds all options for the OpenTelemetry tracer provider.
type Config struct {
	// ServiceName is the logical name of the service (required).
	ServiceName string
	// ServiceVersion is injected as a resource attribute.
	ServiceVersion string
	// Environment is the deployment environment ("production", "staging", etc.).
	Environment string

	// OTLPEndpoint is the host:port of the OTLP gRPC collector (e.g. Jaeger
	// all-in-one or OpenTelemetry Collector). Defaults to "localhost:4317".
	OTLPEndpoint string

	// Insecure controls whether the gRPC connection uses TLS. Set to true for
	// local / in-cluster deployments without TLS.
	Insecure bool

	// SampleRate is a value between 0.0 and 1.0 that controls the fraction of
	// traces that are recorded. Defaults to 1.0 (sample everything).
	SampleRate float64

	// BatchTimeout is the maximum duration before a batch of spans is exported.
	// Defaults to 5 s.
	BatchTimeout time.Duration

	// MaxExportBatchSize limits how many spans are exported in a single request.
	// Defaults to 512.
	MaxExportBatchSize int

	// ConnectTimeout is the timeout for the initial gRPC connection.
	ConnectTimeout time.Duration

	// ExportTimeout is the per-export call timeout. Defaults to 30 s.
	ExportTimeout time.Duration
}

func (c *Config) defaults() {
	if c.OTLPEndpoint == "" {
		c.OTLPEndpoint = "localhost:4317"
	}
	if c.SampleRate <= 0 || c.SampleRate > 1.0 {
		c.SampleRate = 1.0
	}
	if c.BatchTimeout == 0 {
		c.BatchTimeout = 5 * time.Second
	}
	if c.MaxExportBatchSize == 0 {
		c.MaxExportBatchSize = 512
	}
	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	if c.ExportTimeout == 0 {
		c.ExportTimeout = 30 * time.Second
	}
}

// TracerProvider wraps the OpenTelemetry SDK TracerProvider and owns the
// lifecycle of its gRPC connection to the collector.
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	conn     *grpc.ClientConn
}

// NewTracerProvider creates, configures, and registers a global OTel tracer
// provider that exports spans via OTLP/gRPC.
func NewTracerProvider(ctx context.Context, cfg Config) (*TracerProvider, error) {
	cfg.defaults()

	if cfg.ServiceName == "" {
		return nil, errors.New("telemetry: ServiceName is required")
	}

	// ---- gRPC connection to the collector -----------------------------------
	dialCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
	}
	if cfg.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(dialCtx, cfg.OTLPEndpoint, dialOpts...) //nolint:staticcheck // grpc.DialContext is the idiomatic pre-1.63 API
	if err != nil {
		return nil, fmt.Errorf("telemetry: connecting to OTLP endpoint %q: %w", cfg.OTLPEndpoint, err)
	}

	// ---- OTLP exporter -------------------------------------------------------
	exporterOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithGRPCConn(conn),
		otlptracegrpc.WithTimeout(cfg.ExportTimeout),
	}
	exporter, err := otlptracegrpc.New(ctx, exporterOpts...)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("telemetry: creating OTLP exporter: %w", err)
	}

	// ---- Resource (service metadata) -----------------------------------------
	res, err := buildResource(ctx, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("telemetry: building resource: %w", err)
	}

	// ---- Sampler -------------------------------------------------------------
	var sampler sdktrace.Sampler
	switch {
	case cfg.SampleRate >= 1.0:
		sampler = sdktrace.AlwaysSample()
	case cfg.SampleRate <= 0:
		sampler = sdktrace.NeverSample()
	default:
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// ---- Batch span processor ------------------------------------------------
	bsp := sdktrace.NewBatchSpanProcessor(exporter,
		sdktrace.WithBatchTimeout(cfg.BatchTimeout),
		sdktrace.WithMaxExportBatchSize(cfg.MaxExportBatchSize),
		sdktrace.WithExportTimeout(cfg.ExportTimeout),
	)

	// ---- Provider ------------------------------------------------------------
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// Register as global provider so otel.Tracer() works everywhere.
	otel.SetTracerProvider(tp)

	// Register W3C TraceContext + Baggage propagators.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracerProvider{provider: tp, conn: conn}, nil
}

// Shutdown flushes pending spans and releases all resources. It must be called
// before the process exits — typically via `defer tp.Shutdown(ctx)`.
func (t *TracerProvider) Shutdown(ctx context.Context) error {
	var errs []error
	if err := t.provider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("telemetry: shutting down provider: %w", err))
	}
	if t.conn != nil {
		if err := t.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("telemetry: closing gRPC connection: %w", err))
		}
	}
	return errors.Join(errs...)
}

// Provider returns the underlying *sdktrace.TracerProvider for advanced use.
func (t *TracerProvider) Provider() *sdktrace.TracerProvider { return t.provider }

// Tracer returns a named tracer from this provider.
func (t *TracerProvider) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	return t.provider.Tracer(name, opts...)
}

// ---- Global convenience helpers -------------------------------------------

// Start is a thin wrapper over otel.Tracer(tracerName).Start that uses the
// globally registered provider. It is intended for use inside service code
// that does not hold a direct reference to a TracerProvider.
func Start(ctx context.Context, tracerName, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, spanName, opts...)
}

// SpanFromContext returns the active span from ctx (may be a no-op span if
// tracing is not active).
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddEvent adds a named event with optional attributes to the span in ctx.
func AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	trace.SpanFromContext(ctx).AddEvent(name, trace.WithAttributes(attrs...))
}

// RecordError records err on the active span in ctx and sets the span status
// to Error.
func RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	if err == nil {
		return
	}
	trace.SpanFromContext(ctx).RecordError(err, opts...)
}

// SetAttributes sets key-value attributes on the active span.
func SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	trace.SpanFromContext(ctx).SetAttributes(attrs...)
}

// ---- Noop provider (for tests / local dev without a collector) -----------

// NewNoopTracerProvider installs a no-op tracer provider as the global and
// returns a TracerProvider whose Shutdown is a no-op. Useful in tests.
func NewNoopTracerProvider() *TracerProvider {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return &TracerProvider{provider: tp}
}

// ---- internal helpers -----------------------------------------------------

func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
	}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.ServiceVersion))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironment(cfg.Environment))
	}

	return resource.New(ctx,
		resource.WithFromEnv(),          // OTEL_RESOURCE_ATTRIBUTES env var
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithAttributes(attrs...),
	)
}
