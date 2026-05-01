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

	shared_logger "github.com/ichinosekei/highload/internal/logger"
	"github.com/ichinosekei/highload/services/payment/internal/app"
	"github.com/ichinosekei/highload/services/payment/internal/config"
	payment_http "github.com/ichinosekei/highload/services/payment/internal/delivery/http"
	"github.com/ichinosekei/highload/services/payment/internal/platform"
	"github.com/ichinosekei/highload/services/payment/internal/repository"
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

	logger := shared_logger.NewLogger(cfg.Env, "payment")

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

	natsClient := app.MustNewNats(ctx, cfg, logger)
	defer natsClient.Close()

	// --- Repositories ---
	paymentRepo := repository.NewPaymentRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	publisher := repository.NewNatsPublisher(natsClient)
	pspClient := platform.NewMockPSPClient()

	// --- Handlers ---
	h := payment_http.NewHandler(paymentRepo, orderRepo, pspClient, publisher, logger)

	// --- Router ---
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(serverTimeout))

	r.Route("/api/v1", h.RegisterRoutes)

	// --- Server start ---
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "payment service started", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf("server startup: %w", err)
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-ctx.Done():
		logger.InfoContext(context.Background(), "shutting down payment service")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("force server shutdown: %w", err)
		}

		logger.InfoContext(context.Background(), "server stopped")
		return nil
	}
}
