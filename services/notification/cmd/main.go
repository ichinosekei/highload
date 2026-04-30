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

	logger := core_logger.NewLogger(cfg.Env, "notification")

	if errRun := run(&cfg, logger); errRun != nil {
		logger.Error("application startup", "error", errRun)
		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *slog.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// --- Инициализация компонентов ---
	natsClient := app.MustNewNats(ctx, cfg, logger)
	defer natsClient.Close()

	// --- Провайдеры ---
	mockSender := providers.NewMockSender(logger)

	// --- Сервис ---
	svc := app.NewService(mockSender, logger)

	// --- Обработчики (Consumer) ---
	consumer := notif_nats.NewConsumer(natsClient, svc, logger)
	if err := consumer.Start(ctx); err != nil {
		return fmt.Errorf("start nats consumer: %w", err)
	}

	// --- Health check server ---
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(serverTimeout))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "notification service health-check server started", "port", cfg.Port)
		if errServe := srv.ListenAndServe(); errServe != nil && errServe != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("health server startup: %w", errServe)
		}
	}()

	select {
	case errSrv := <-serverErrors:
		return errSrv
	case <-ctx.Done():
		logger.InfoContext(context.Background(), "shutting down notification service")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if errShutdown := srv.Shutdown(shutdownCtx); errShutdown != nil {
			return fmt.Errorf("force server shutdown: %w", errShutdown)
		}

		logger.InfoContext(context.Background(), "service stopped")
		return nil
	}
}
