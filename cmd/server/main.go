package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"certificate-inspector/internal/api"
	"certificate-inspector/internal/config"
	"certificate-inspector/internal/observability"
	"certificate-inspector/internal/store"
	"certificate-inspector/internal/worker"
)

func main() {
	logger := observability.NewLogger(os.Stdout)
	cfg := config.FromEnv()

	db, err := store.OpenSQLite(cfg.DatabasePath)
	if err != nil {
		logger.Error("open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(cfg.MigrationsPath); err != nil {
		logger.Error("run migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	queue := worker.NewQueue(cfg.QueueSize)
	inspector := worker.NewCertificateInspector(cfg.InspectionTimeout)
	processor := worker.NewProcessor(db, queue, inspector, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	processor.Start(ctx)

	router := api.NewRouter(api.Dependencies{
		Store:  db,
		Queue:  queue,
		Logger: logger,
	})

	server := &http.Server{
		Addr:         cfg.ListenAddress,
		Handler:      router,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}

	go func() {
		logger.Info("server starting", slog.String("addr", cfg.ListenAddress))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", slog.String("error", err.Error()))
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown requested")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", slog.String("error", err.Error()))
	}
	processor.Stop(shutdownCtx)
}
