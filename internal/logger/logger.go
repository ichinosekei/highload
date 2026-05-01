package logger

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

func NewLogger(env, serviceName string) *slog.Logger {
	var handler slog.Handler

	level := slog.LevelInfo
	if env == "local" {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}

	switch env {
	case "prod":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "test":
		handler = slog.DiscardHandler
	default:
		handler = tint.NewHandler(os.Stdout, &tint.Options{Level: level})
	}

	return slog.New(handler).With(slog.String("service", serviceName))
}
