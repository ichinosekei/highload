package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	shared_handler "github.com/ichinosekei/highload/internal/delivery/http"
	shared_logger "github.com/ichinosekei/highload/internal/logger"
	"github.com/ichinosekei/highload/services/catalog/internal/app"
	"github.com/ichinosekei/highload/services/catalog/internal/config"
	catalog_http "github.com/ichinosekei/highload/services/catalog/internal/delivery/http"
	"github.com/ichinosekei/highload/services/catalog/internal/repository"
)

const (
	serverTimeout     = 60 * time.Second
	shutdownTimeout   = 5 * time.Second
	readHeaderTimeout = 3 * time.Second
)

func main() {
	cfg, err := env.ParseAs[config.Config]()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize configuration: %v", err))
	}

	logger := shared_logger.NewLogger(cfg.Env, "catalog")

	if err := run(&cfg, logger); err != nil {
		logger.Error("application startup", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Infra initialization ---
	db := app.MustNewPostgres(ctx, cfg, logger)
	defer db.Close()

	meiliClient := app.MustNewMeili(ctx, cfg, logger)
	defer meiliClient.Close()

	rdb := app.MustNewRedis(ctx, cfg, logger)
	defer rdb.Close()

	// --- Repositories ---
	resRepo := repository.NewRestaurantRepository(db)
	menuRepo := repository.NewMenuRepository(db)
	searchRepo := repository.NewSearchRepository(meiliClient)

	// --- Caching Decorators ---
	cachedSearchRepo := repository.NewSearchCacheDecorator(searchRepo, rdb, cfg.CacheTTLSearch)
	cachedMenuResRepo := repository.NewRestaurantCacheDecorator(
		repository.MenuRestaurantComposite{
			RestaurantReader: resRepo,
			MenuReader:       menuRepo,
		},
		rdb,
		cfg.CacheTTLMenu,
	)

	// --- Handlers ---
	h := catalog_http.NewHandler(cachedMenuResRepo, cachedSearchRepo, logger)

	// --- Initial sync for local development ---
	if cfg.Env == "local" {
		const syncLimit = 1000
		restaurants, errSync := resRepo.List(ctx, syncLimit, 0)
		if errSync == nil {
			if err := searchRepo.Sync(ctx, restaurants); err != nil {
				logger.WarnContext(ctx, "initial search sync failed", "error", err)
			} else {
				logger.InfoContext(ctx, "initial search sync completed", "count", len(restaurants))
			}
		}
	}

	// --- Router ---
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(serverTimeout))

	r.Get("/health", shared_handler.HealthCheck)

	r.Route("/api/v1", h.RegisterRoutes)

	// --- Server start ---
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "catalog service started", "port", cfg.Port)
		if errServe := srv.ListenAndServe(); errServe != nil && errServe != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("server startup: %w", errServe)
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		logger.InfoContext(context.Background(), "shutting down catalog service")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if errShutdown := srv.Shutdown(shutdownCtx); errShutdown != nil {
			return fmt.Errorf("force server shutdown: %w", errShutdown)
		}

		logger.InfoContext(context.Background(), "server stopped")
		return nil
	}
}
