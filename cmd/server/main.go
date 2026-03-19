package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/config"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/http-server/handlers/monitor"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/http-server/middleware/logger"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/handlers/slogpretty"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/sl"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/services/checker"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/storage/memory"

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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// TODO: DB & Storage
	// storage, err := sqlite.New(cfg.StoragePath)
	// if err != nil {
	// 	log.Error("failed to initialize storage", sl.Err(err))
	// 	os.Exit(1)
	// }

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

	mHandler := monitor.NewHandler(log, val, storage, storage)

	router.Route("/monitors", func(r chi.Router) {
		r.Post("/", mHandler.Save)
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

	testMonitor, err := storage.GetMonitor(context.Background(), 1)
	_ = err

	duration, err := checker.CheckTCP(checker.CheckRequest{URL: testMonitor.URL})
	log.Debug(fmt.Sprintf("CheckTCP %s in %s", testMonitor.URL, duration))

	// TODO: Graceful shutdown
	sign := <-stop
	log.Info("Stopping server", slog.String("signal", sign.String()))
	fmt.Println()
	log.Info("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPServer.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
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
