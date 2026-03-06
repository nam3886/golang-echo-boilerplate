package observability

import (
	"log/slog"
	"os"
	"strings"

	"github.com/gnha/gnha-services/internal/shared/config"
)

// NewLogger creates a structured logger based on environment.
func NewLogger(cfg *config.Config) *slog.Logger {
	level := parseLevel(cfg.LogLevel)
	var handler slog.Handler

	if cfg.IsDevelopment() {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
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
