package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quotes-service/internal/config"
	approuter "quotes-service/internal/http-server/router"
	"quotes-service/internal/lib/logger/sl"
	"quotes-service/internal/storage/memorystorage"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
	defaulTimeout = 10 * time.Second
)

func main() {
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	log.Info(
		"starting quote-service",
		slog.String("env", cfg.Env),
		slog.String("version", cfg.Version),
	)
	log.Debug("debug messages are enabled")

	storage, err := memorystorage.New()
	if err != nil {
		log.Error("failed to init storage", sl.Err(err))
		os.Exit(1)
	}
	defer func() {
		log.Info("closing storage")
		if err := storage.Close(); err != nil {
			log.Error("failed to close storage", sl.Err(err))
		}
	}()

	mainRouter := approuter.New(log, storage)

	log.Info("starting server", slog.String("address", cfg.HTTPServer.Address))

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      mainRouter,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("failed to start server", sl.Err(err))
			os.Exit(1)
		}
	}()

	log.Info("server started and listening for quote requests")

	<-done
	log.Info("stopping server")

	shutdownTimeout := defaulTimeout
	if cfg.HTTPServer.Timeout > 0 {
		shutdownTimeout = cfg.HTTPServer.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("failed to stop server gracefully", sl.Err(err))
	}

	log.Info("server stopped")
}

func setupLogger(env string) *slog.Logger {
	var handler slog.Handler

	switch env {
	case envLocal:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	case envDev:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	case envProd:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	default:
		defaultLevel := slog.LevelInfo
		tempLogHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: defaultLevel})
		tempLogger := slog.New(tempLogHandler)
		tempLogger.Warn("Invalid 'env' in config, defaulting to 'prod' logger settings (Info level).", slog.String("configured_env", env))
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: defaultLevel})
	}
	return slog.New(handler)
}