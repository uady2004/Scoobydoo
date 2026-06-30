// Package logger provides a one-call bootstrap for the shared logger that reads
// configuration from environment variables, so each microservice main() can
// initialise a production-ready logger in a single line.
package logger

import (
	"os"
	"strings"

	pkglogger "github.com/tiktok-clone/shared/pkg/logger"
	"go.uber.org/zap/zapcore"
)

// FromEnv creates a *pkglogger.Logger configured from environment variables:
//
//	LOG_LEVEL   — debug | info | warn | error  (default: info)
//	LOG_FORMAT  — json | console               (default: json)
//	SERVICE_NAME    — injected as a static "service" field
//	SERVICE_VERSION — injected as a static "version" field
func FromEnv() (*pkglogger.Logger, error) {
	lvl := parseLevel(os.Getenv("LOG_LEVEL"))
	enc := os.Getenv("LOG_FORMAT")
	if enc == "" {
		enc = "json"
	}

	return pkglogger.New(pkglogger.Config{
		Level:          lvl,
		Encoding:       enc,
		ServiceName:    os.Getenv("SERVICE_NAME"),
		ServiceVersion: os.Getenv("SERVICE_VERSION"),
		Development:    strings.ToLower(os.Getenv("APP_ENV")) == "development",
	})
}

// MustFromEnv is like FromEnv but panics on error.
func MustFromEnv() *pkglogger.Logger {
	l, err := FromEnv()
	if err != nil {
		panic("logger: failed to initialise from env: " + err.Error())
	}
	return l
}

// Development creates a logger suitable for local development:
// console format, debug level, caller/stack-trace enabled.
func Development(serviceName string) (*pkglogger.Logger, error) {
	return pkglogger.New(pkglogger.Config{
		Level:       pkglogger.DebugLevel,
		Encoding:    "console",
		ServiceName: serviceName,
		Development: true,
	})
}

// Production creates a logger suitable for production deployments:
// JSON format, info level, no stack traces.
func Production(serviceName, serviceVersion string) (*pkglogger.Logger, error) {
	return pkglogger.New(pkglogger.Config{
		Level:          pkglogger.InfoLevel,
		Encoding:       "json",
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Development:    false,
	})
}

// parseLevel converts a string level name to a zapcore.Level.
// Defaults to InfoLevel for unrecognised values.
func parseLevel(s string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return pkglogger.DebugLevel
	case "warn", "warning":
		return pkglogger.WarnLevel
	case "error":
		return pkglogger.ErrorLevel
	case "fatal":
		return pkglogger.FatalLevel
	default:
		return pkglogger.InfoLevel
	}
}
