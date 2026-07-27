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

	// "time"

	"github.com/vdzhagaev/watchlight/internal/config"
	"github.com/vdzhagaev/watchlight/internal/http-server/handlers/monitorhandler"
	"github.com/vdzhagaev/watchlight/internal/http-server/middleware/logger"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/handlers/slogpretty"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/services/checker"
	"github.com/vdzhagaev/watchlight/internal/services/scheduler"

	// "github.com/vdzhagaev/watchlight/internal/services/checker"
	// "github.com/vdzhagaev/watchlight/internal/services/scheduler"
	"github.com/vdzhagaev/watchlight/internal/storage/sqlite"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"

	EVENTS_CHANNEL_SIZE = 100
	SCHEDULER_WORKERS   = 50
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	// TODO: Config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	// TODO: Logger
	log := setupLogger(cfg.Env)
	log.Info("Uptime Monitoring Service starting...")

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// TODO: DB & Storage
	storage, err := sqlite.New(cfg.StoragePath)
	if err != nil {
		return fmt.Errorf("failed to init storage: %w", err)
	}

	val := validator.New()

	eventsChan := make(chan monitor.ConfigChangeEvent, EVENTS_CHANNEL_SIZE)

	mService := monitor.NewService(storage, log, eventsChan)

	mHandler := monitorhandler.NewHandler(log, val, mService)

	// TODO: Workers: Scheduler & Checker
	scheduler := scheduler.New(scheduler.Params{
		Logger:  log,
		Getter:  storage,
		Handler: mService,
		Workers: SCHEDULER_WORKERS,
		Checkers: map[monitor.CheckType]checker.Checker{
			monitor.CheckPing: checker.TCPChecker{},
			monitor.CheckHTTP: checker.HTTPChecker{},
		},
		WriteTimeout: 5 * time.Second,
		ConfigEvents: eventsChan,
	})

	err = scheduler.Start(appCtx)
	if err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(logger.New(log))
	router.Use(middleware.Recoverer)
	router.Use(middleware.URLFormat)

	router.Route("/monitors", func(r chi.Router) {
		r.Post("/", mHandler.Save)
		r.Patch("/{monitorID}", mHandler.Patch)
		r.Delete("/{monitorID}", mHandler.Delete)
		r.Get("/", mHandler.List)
		r.Get("/{monitorID}", mHandler.Find)

		r.Patch("/{monitorID}/ping", mHandler.UpdatePing)

		r.Route("/{monitorID}/http-checks", func(r chi.Router) {
			r.Post("/", mHandler.AddHTTPCheck)
			r.Patch("/{configID}", mHandler.UpdateHTTPCheck)
			r.Delete("/{configID}", mHandler.RemoveHTTPCheck)
		})
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

	if err := scheduler.Stop(shutdownCtx); err != nil {
		log.Warn("scheduler drain timed out", sl.Err(err))
	}

	defer storage.Close()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Info("Server stopped")
	return nil
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
