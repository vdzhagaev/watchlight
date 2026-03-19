package monitor

import (
	"context"
	"log/slog"

	"github.com/go-playground/validator/v10"
	"gitlab.com/l0veme-projects/uptime-monitor/internal/domain"
)

type MonitorFinder interface {
	GetMonitor(ctx context.Context, id int64) (domain.Monitor, error)
	GetMonitorList(ctx context.Context) ([]domain.Monitor, error)
}

type MonitorSaver interface {
	SaveMonitor(ctx context.Context, m *domain.Monitor) error
}

type MonitorHandler struct {
	log    *slog.Logger
	val    *validator.Validate
	finder MonitorFinder
	saver  MonitorSaver
}

func NewHandler(log *slog.Logger, v *validator.Validate, finder MonitorFinder, saver MonitorSaver) *MonitorHandler {
	return &MonitorHandler{
		log:    log,
		val:    v,
		finder: finder, // передаем один и тот же объект, но по разным "ролям"
		saver:  saver,
	}
}
