package logger

import (
	"log/slog"
	"os"
)

// NewLogger creates a new logger with the specified level.
func NewLogger(level string) *slog.Logger {
	opts := &slog.HandlerOptions{}

	switch level {
	case "debug":
		opts.Level = slog.LevelDebug
	case "info":
		opts.Level = slog.LevelInfo
	case "warn":
		opts.Level = slog.LevelWarn
	case "error":
		opts.Level = slog.LevelError
	default:
		opts.Level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	return slog.New(handler)
}
