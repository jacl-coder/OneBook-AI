package util

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type loggerContextKey struct{}

// LoggerFromContext returns the *slog.Logger stored in ctx.
// Falls back to slog.Default() when no logger is present.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok {
			return l
		}
	}
	return slog.Default()
}

// ContextWithLogger returns a child context carrying the given logger.
func ContextWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, l)
}

// parseLevel converts a string level name into slog.Level.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// openLogFile creates (or appends to) a date-stamped log file under logsDir.
// The file is named <service>-<YYYY-MM-DD>.log.
// Returns the file and a cleanup function. Caller must defer cleanup.
func openLogFile(logsDir, service string) (*os.File, error) {
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs dir %s: %w", logsDir, err)
	}
	name := fmt.Sprintf("%s-%s.log", service, time.Now().Format("2006-01-02"))
	fpath := filepath.Join(logsDir, name)
	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", fpath, err)
	}
	return f, nil
}

// InitLogger configures the global slog logger.
// Logs are written as JSON to both stdout and files under logsDir (if non-empty).
// Two files are created:
//   - <service>-<YYYY-MM-DD>.log  – per-service log
//   - all-<YYYY-MM-DD>.log        – combined log for all services
//
// Returns the logger and an optional cleanup function (nil when no file is opened).
func InitLogger(level, service, logsDir string) (*slog.Logger, func()) {
	slogLevel := parseLevel(level)

	var w io.Writer = os.Stdout
	var cleanup func()

	if logsDir != "" {
		var writers []io.Writer
		writers = append(writers, os.Stdout)
		var closers []func()

		// Per-service log file
		sf, err := openLogFile(logsDir, service)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: failed to open service log file: %v\n", err)
		} else {
			writers = append(writers, sf)
			closers = append(closers, func() { _ = sf.Close() })
		}

		// Combined log file for all services
		af, err := openLogFile(logsDir, "all")
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: failed to open combined log file: %v\n", err)
		} else {
			writers = append(writers, af)
			closers = append(closers, func() { _ = af.Close() })
		}

		if len(writers) > 1 {
			w = io.MultiWriter(writers...)
			cleanup = func() {
				for _, c := range closers {
					c()
				}
			}
		}
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:     slogLevel,
		AddSource: true,
	})
	logger := slog.New(handler).With("service", service)
	slog.SetDefault(logger)
	return logger, cleanup
}

// Fatal logs at Error level and exits the process.
// Use instead of log.Fatalf so all output goes through slog JSON formatting.
func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
