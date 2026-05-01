package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ichinosekei/highload/services/catalog/internal/config"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
)

func MustNewMeili(ctx context.Context, cfg *config.Config, logger *slog.Logger) *platform.MeiliClient {
	meiliClient, err := platform.NewMeiliClient(cfg.MeiliHost, cfg.MeiliKey)
	if err != nil {
		panic(fmt.Sprintf("meilisearch connection failed: %v", err))
	}

	if err := meiliClient.InitIndices(ctx); err != nil {
		logger.WarnContext(ctx, "meilisearch indices initialization", "error", err)
	}

	logger.InfoContext(ctx, "meilisearch connection established")
	return meiliClient
}
