package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/config"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/lib/logger/handlers/slogpretty"
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
	// TODO: Workers: Scheduler & Checker
	// TODO: HTTP Server
	// TODO: Graceful shutdown

	log.Info("Monitoring active. Press Ctrl+C to stop the server.")

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
