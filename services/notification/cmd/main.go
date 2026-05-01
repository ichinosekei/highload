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
	"github.com/ichinosekei/highload/services/notification/internal/app"
	"github.com/ichinosekei/highload/services/notification/internal/config"
	notif_nats "github.com/ichinosekei/highload/services/notification/internal/delivery/nats"
	"github.com/ichinosekei/highload/services/notification/internal/platform/providers"
)

const (
	serverTimeout     = 30 * time.Second
	shutdownTimeout   = 5 * time.Second
	readHeaderTimeout = 3 * time.Second
)

func main() {
	cfg, err := env.ParseAs[config.Config]()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize configuration: %v", err))
	}

	logger := shared_logger.NewLogger(cfg.Env, "notification")

	if err := run(&cfg, logger); err != nil {
		logger.Error("application startup", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Infra initialization ---
	natsClient := app.MustNewNats(ctx, cfg, logger)
	defer natsClient.Close()

	// --- Providers ---
	mockSender := providers.NewMockSender(logger)

	// --- Services ---
	svc := app.NewService(mockSender, logger)

	// --- Consumers ---
	consumer := notif_nats.NewConsumer(natsClient, svc, logger)
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("start nats consumer: %w", err)
	}

	// --- Router ---
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(serverTimeout))

	r.Get("/health", shared_handler.HealthCheck)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "notification service health-check server started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("health server startup: %w", err)
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		logger.InfoContext(context.Background(), "shutting down notification service")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("force server shutdown: %w", err)
		}

		logger.InfoContext(context.Background(), "service stopped")
		return nil
	}
}
