package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ichinosekei/highload/services/payment/internal/config"
	"github.com/ichinosekei/highload/services/payment/internal/platform"
)

func MustNewPostgres(ctx context.Context, cfg *config.Config, logger *slog.Logger) *platform.PostgresDB {
	db, err := platform.NewPostgresDB(ctx, cfg.PostgresURL)
	if err != nil {
		panic(fmt.Sprintf("postgres connection failed: %v", err))
	}

	logger.InfoContext(ctx, "postgres connection established")
	return db
}
