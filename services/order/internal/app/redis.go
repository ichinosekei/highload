package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ichinosekei/highload/services/order/internal/config"
	"github.com/ichinosekei/highload/services/order/internal/platform"
)

func MustNewRedis(ctx context.Context, cfg *config.Config, logger *slog.Logger) *platform.RedisClient {
	rdb, err := platform.NewRedisClient(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		panic(fmt.Sprintf("redis connection failed: %v", err))
	}

	logger.InfoContext(ctx, "redis connection established")
	return rdb
}
