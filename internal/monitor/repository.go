package monitor

import (
	"context"
)

type Repositoty interface {
	GetMonitor(ctx context.Context, id int64) (Monitor, error)
	GetMonitorList(ctx context.Context) ([]Monitor, error)
	SaveMonitor(ctx context.Context, in CreateMonitorInput) (Monitor, error)
	UpdateMonitor(ctx context.Context, id int64, in UpdateMonitorInput) (Monitor, error)
	DeleteMonitor(ctx context.Context, id int64) error
}
