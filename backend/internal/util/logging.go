package util

import (
	"log/slog"
	"os"
)

// InitLogger configures the global slog logger with JSON output and level.
// Accepts levels: debug, info, warn, error. Defaults to info on unknown input.
func InitLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slogLevel,
		AddSource: true,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
