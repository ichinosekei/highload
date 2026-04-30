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

	core_logger "github.com/ichinosekei/highload/internal/logger"
	"github.com/ichinosekei/highload/services/order/internal/app"
	"github.com/ichinosekei/highload/services/order/internal/config"
	order_http "github.com/ichinosekei/highload/services/order/internal/delivery/http"
	"github.com/ichinosekei/highload/services/order/internal/repository"
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

	logger := core_logger.NewLogger(cfg.Env, "order")

	if err := run(&cfg, logger); err != nil {
		logger.Error("application startup", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Инициализация компонентов ---
	db := app.MustNewPostgres(ctx, cfg, logger)
	defer db.Close()

	rdb := app.MustNewRedis(ctx, cfg, logger)
	defer rdb.Close()

	natsClient := app.MustNewNats(ctx, cfg, logger)
	defer natsClient.Close()

	// --- Репозитории ---
	orderRepo := repository.NewOrderRepository(db)
	cartRepo := repository.NewCartRepository(rdb)
	publisher := repository.NewNatsPublisher(natsClient)

	// --- Обработчики ---
	h := order_http.NewHandler(orderRepo, cartRepo, publisher, logger)

	// --- Роутер ---
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(serverTimeout))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	r.Route("/api/v1", h.RegisterRoutes)

	// --- Запуск сервера ---
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "order service started", "port", cfg.Port)
		if errServe := srv.ListenAndServe(); errServe != nil && errServe != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("server startup: %w", errServe)
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		logger.InfoContext(context.Background(), "shutting down order service")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if errShutdown := srv.Shutdown(shutdownCtx); errShutdown != nil {
			return fmt.Errorf("force server shutdown: %w", errShutdown)
		}

		logger.InfoContext(context.Background(), "server stopped")
		return nil
	}
}
