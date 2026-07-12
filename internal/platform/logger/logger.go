package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New creates a structured JSON logger writing to stdout at the given level.
// Unknown or empty levels default to info.
func New(level string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	}))
}

// WithCorrelation returns a logger enriched with correlation_id when id is set.
func WithCorrelation(logger *slog.Logger, id string) *slog.Logger {
	if logger == nil || id == "" {
		return logger
	}
	return logger.With("correlation_id", id)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
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
