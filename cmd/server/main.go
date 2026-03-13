package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/config"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/http-server/handlers/monitor/save"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/http-server/middleware/logger"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/handlers/slogpretty"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/sl"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/storage/sqlite"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// TODO: DB & Storage
	storage, err := sqlite.New(cfg.StoragePath)
	if err != nil {
		log.Error("failed to initialize storage", sl.Err(err))
		os.Exit(1)
	}

	_ = storage

	// TODO: Workers: Scheduler & Checker
	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(logger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	router.Post("/sites", save.New(log,
		storage))

	// TODO: HTTP Server
	server := http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      router,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	log.Info("HTTP server started", slog.String("address", cfg.HTTPServer.Address))

	log.Info("Monitoring active. Press Ctrl+C to stop the server.")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("failed to start HTTP server", sl.Err(err))
	}

	// TODO: Graceful shutdown
	<-stop
	fmt.Println()
	log.Info("Shutting down gracefully...")
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
