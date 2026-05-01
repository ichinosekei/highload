package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ichinosekei/highload/services/catalog/internal/config"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
)

func MustNewRedis(ctx context.Context, cfg *config.Config, logger *slog.Logger) *platform.RedisClient {
	rdb, err := platform.NewRedisClient(ctx, cfg.RedisAddr)
	if err != nil {
		panic(fmt.Sprintf("redis connection failed: %v", err))
	}

	logger.InfoContext(ctx, "redis connection established")
	return rdb
}
