package monitor

import (
	"context"

	"gitlab.com/l0veme-projects/uptime-monitor/internal/domain"
)

type MonitorFinder interface {
	GetMonitor(ctx context.Context, id int64) (domain.Monitor, error)
	GetMonitorList(ctx context.Context) ([]domain.Monitor, error)
}

type MonitorSaver interface {
	SaveMonitor(ctx context.Context, m *domain.Monitor) error
}
