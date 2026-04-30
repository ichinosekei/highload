package app

import (
	"context"
	"log/slog"

	"github.com/ichinosekei/highload/services/order/internal/config"
	"github.com/ichinosekei/highload/services/order/internal/platform"
)

func MustNewNats(ctx context.Context, cfg *config.Config, logger *slog.Logger) *platform.NatsClient {
	client, err := platform.NewNatsClient(cfg.NatsURL)
	if err != nil {
		logger.ErrorContext(ctx, "failed to connect to nats", "error", err)
		panic(err)
	}

	// Ensure our stream exists for order events.
	errStream := client.EnsureStream(ctx, "NOTIFICATIONS", []string{"order.*", "payment.*"})
	if errStream != nil {
		logger.ErrorContext(ctx, "failed to ensure nats stream", "error", errStream)
		panic(errStream)
	}

	logger.InfoContext(ctx, "nats connection established")
	return client
}
