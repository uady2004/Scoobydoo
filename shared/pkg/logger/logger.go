// Package logger provides a structured logger built on uber-go/zap with
// automatic OpenTelemetry trace/span ID injection into every log entry.
package logger

import (
	"context"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Level aliases zap log levels so callers need not import zap directly.
type Level = zapcore.Level

const (
	DebugLevel = zapcore.DebugLevel
	InfoLevel  = zapcore.InfoLevel
	WarnLevel  = zapcore.WarnLevel
	ErrorLevel = zapcore.ErrorLevel
	FatalLevel = zapcore.FatalLevel
)

// Config holds the configuration for the logger.
type Config struct {
	// Level is the minimum log level to emit. Defaults to InfoLevel.
	Level Level
	// Development enables caller/stack-trace information and pretty output.
	Development bool
	// Encoding is "json" (default) or "console".
	Encoding string
	// OutputPaths is a list of URLs or file paths to write logs to.
	// Defaults to ["stdout"].
	OutputPaths []string
	// ServiceName is injected as a static "service" field on every entry.
	ServiceName string
	// ServiceVersion is injected as a static "version" field on every entry.
	ServiceVersion string
}

// Logger wraps a *zap.Logger and exposes context-aware helpers that
// automatically inject the OpenTelemetry trace ID and span ID.
type Logger struct {
	zap     *zap.Logger
	sugar   *zap.SugaredLogger
	mu      sync.RWMutex
	level   zap.AtomicLevel
	cfg     Config
}

var (
	globalLogger *Logger
	once         sync.Once
)

// New creates a Logger from the given Config. The first call to New also
// sets the package-level global logger returned by Global().
func New(cfg Config) (*Logger, error) {
	if cfg.Encoding == "" {
		cfg.Encoding = "json"
	}
	if len(cfg.OutputPaths) == 0 {
		cfg.OutputPaths = []string{"stdout"}
	}

	atomicLevel := zap.NewAtomicLevelAt(cfg.Level)

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339Nano)
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
	encoderCfg.EncodeDuration = zapcore.MillisDurationEncoder
	if cfg.Development {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	zapCfg := zap.Config{
		Level:             atomicLevel,
		Development:       cfg.Development,
		Encoding:          cfg.Encoding,
		EncoderConfig:     encoderCfg,
		OutputPaths:       cfg.OutputPaths,
		ErrorOutputPaths:  []string{"stderr"},
		DisableCaller:     false,
		DisableStacktrace: !cfg.Development,
	}

	base, err := zapCfg.Build(
		zap.AddCallerSkip(1),
		zap.Fields(staticFields(cfg)...),
	)
	if err != nil {
		return nil, err
	}

	l := &Logger{
		zap:   base,
		sugar: base.Sugar(),
		level: atomicLevel,
		cfg:   cfg,
	}

	once.Do(func() { globalLogger = l })
	return l, nil
}

// MustNew is like New but panics on error.
func MustNew(cfg Config) *Logger {
	l, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return l
}

// Global returns the package-level logger. If New has never been called,
// Global initialises a sensible default (JSON, Info, stdout).
func Global() *Logger {
	if globalLogger == nil {
		once.Do(func() {
			l, _ := New(Config{
				Level:       InfoLevel,
				Encoding:    "json",
				ServiceName: os.Getenv("SERVICE_NAME"),
			})
			globalLogger = l
		})
	}
	return globalLogger
}

// SetGlobal replaces the package-level global logger.
func SetGlobal(l *Logger) {
	globalLogger = l
}

// SetLevel changes the minimum log level at runtime without restarting.
func (l *Logger) SetLevel(lvl Level) {
	l.level.SetLevel(lvl)
}

// Sync flushes any buffered log entries. Should be called before program exit.
func (l *Logger) Sync() error {
	return l.zap.Sync()
}

// ---- Context-aware helpers -----------------------------------------------

// WithContext returns a child logger enriched with the OpenTelemetry trace ID
// and span ID extracted from ctx. If the context carries no span the original
// logger is returned unchanged.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return l
	}
	sc := span.SpanContext()
	child := l.zap.With(
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
		zap.Bool("trace_sampled", sc.IsSampled()),
	)
	return &Logger{zap: child, sugar: child.Sugar(), level: l.level, cfg: l.cfg}
}

// With returns a child logger that always includes the given fields.
func (l *Logger) With(fields ...zap.Field) *Logger {
	child := l.zap.With(fields...)
	return &Logger{zap: child, sugar: child.Sugar(), level: l.level, cfg: l.cfg}
}

// WithFields returns a child logger with the supplied key-value pairs as fields.
// Keys must be strings; values may be any type (handled by zap.Any).
func (l *Logger) WithFields(kvs ...interface{}) *Logger {
	child := l.sugar.With(kvs...).Desugar()
	return &Logger{zap: child, sugar: child.Sugar(), level: l.level, cfg: l.cfg}
}

// Named returns a child logger with the given name component appended.
func (l *Logger) Named(name string) *Logger {
	child := l.zap.Named(name)
	return &Logger{zap: child, sugar: child.Sugar(), level: l.level, cfg: l.cfg}
}

// ---- Structured log methods -----------------------------------------------

func (l *Logger) Debug(msg string, fields ...zap.Field)  { l.zap.Debug(msg, fields...) }
func (l *Logger) Info(msg string, fields ...zap.Field)   { l.zap.Info(msg, fields...) }
func (l *Logger) Warn(msg string, fields ...zap.Field)   { l.zap.Warn(msg, fields...) }
func (l *Logger) Error(msg string, fields ...zap.Field)  { l.zap.Error(msg, fields...) }
func (l *Logger) Fatal(msg string, fields ...zap.Field)  { l.zap.Fatal(msg, fields...) }

// DebugCtx is Debug enriched with trace information from ctx.
func (l *Logger) DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).zap.Debug(msg, fields...)
}

// InfoCtx is Info enriched with trace information from ctx.
func (l *Logger) InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).zap.Info(msg, fields...)
}

// WarnCtx is Warn enriched with trace information from ctx.
func (l *Logger) WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).zap.Warn(msg, fields...)
}

// ErrorCtx is Error enriched with trace information from ctx.
func (l *Logger) ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).zap.Error(msg, fields...)
}

// ---- Sugared (printf-style) helpers ---------------------------------------

func (l *Logger) Debugf(format string, args ...interface{}) { l.sugar.Debugf(format, args...) }
func (l *Logger) Infof(format string, args ...interface{})  { l.sugar.Infof(format, args...) }
func (l *Logger) Warnf(format string, args ...interface{})  { l.sugar.Warnf(format, args...) }
func (l *Logger) Errorf(format string, args ...interface{}) { l.sugar.Errorf(format, args...) }
func (l *Logger) Fatalf(format string, args ...interface{}) { l.sugar.Fatalf(format, args...) }

// ---- Package-level convenience wrappers -----------------------------------

func Debug(msg string, fields ...zap.Field)  { Global().Debug(msg, fields...) }
func Info(msg string, fields ...zap.Field)   { Global().Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)   { Global().Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field)  { Global().Error(msg, fields...) }
func Fatal(msg string, fields ...zap.Field)  { Global().Fatal(msg, fields...) }

func DebugCtx(ctx context.Context, msg string, fields ...zap.Field) {
	Global().DebugCtx(ctx, msg, fields...)
}
func InfoCtx(ctx context.Context, msg string, fields ...zap.Field) {
	Global().InfoCtx(ctx, msg, fields...)
}
func WarnCtx(ctx context.Context, msg string, fields ...zap.Field) {
	Global().WarnCtx(ctx, msg, fields...)
}
func ErrorCtx(ctx context.Context, msg string, fields ...zap.Field) {
	Global().ErrorCtx(ctx, msg, fields...)
}

// Zap returns the underlying *zap.Logger for direct use with libraries that
// accept *zap.Logger (e.g. grpc-zap middleware).
func (l *Logger) Zap() *zap.Logger { return l.zap }

// ---- internal helpers -----------------------------------------------------

func staticFields(cfg Config) []zap.Field {
	var fields []zap.Field
	if cfg.ServiceName != "" {
		fields = append(fields, zap.String("service", cfg.ServiceName))
	}
	if cfg.ServiceVersion != "" {
		fields = append(fields, zap.String("version", cfg.ServiceVersion))
	}
	hostname, _ := os.Hostname()
	if hostname != "" {
		fields = append(fields, zap.String("host", hostname))
	}
	return fields
}
