package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/vdzhagaev/watchlight/internal/config"
	"github.com/vdzhagaev/watchlight/internal/http-server/handlers/monitorhandler"
	"github.com/vdzhagaev/watchlight/internal/http-server/middleware/logger"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/handlers/slogpretty"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/storage/memory"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	// TODO: Config
	cfg := config.MustLoad()
	// TODO: Logger
	log := setupLogger(cfg.Env)
	log.Info("Uptime Monitoring Service starting...")

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// TODO: DB & Storage
	storage := memory.New()

	// TODO: Workers: Scheduler & Checker
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(logger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	val := validator.New()

	mService := monitor.NewService(storage, log)
	mHandler := monitorhandler.NewHandler(log, val, mService)

	router.Route("/monitors", func(r chi.Router) {
		r.Post("/", mHandler.Save)
		r.Patch("/{monitorID}", mHandler.Patch)
		r.Get("/", mHandler.List)
		r.Get("/{monitorID}", mHandler.Find)
	})

	// TODO: HTTP Server
	server := http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	log.Info("HTTP server started", slog.String("address", cfg.HTTPServer.Address))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("failed to start HTTP server", sl.Err(err))
		}
	}()

	log.Info("Monitoring active. Press Ctrl+C to stop the server.")

	// TODO: Graceful shutdown
	<-appCtx.Done()
	log.Info("Stopping server")
	fmt.Println()
	log.Info("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPServer.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown failed", sl.Err(err))
	}

	if err := storage.Close(); err != nil {
		log.Error("failed to close storage", sl.Err(err))
	}

	log.Info("Server stopped")
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger
	switch env {
	case envLocal:
		log = setupPrettySlog()
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}

	return log
}

func setupPrettySlog() *slog.Logger {
	opts := slogpretty.PrettyHandlerOptions{
		SlogOpts: &slog.HandlerOptions{Level: slog.LevelDebug},
	}

	handler := opts.NewPrettyHandler(os.Stdout)

	return slog.New(handler)
}
