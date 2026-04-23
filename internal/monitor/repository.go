package monitor

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	GetMonitor(ctx context.Context, id uuid.UUID) (Monitor, error)
	GetMonitorList(ctx context.Context) ([]Monitor, error)
	CreateMonitor(ctx context.Context, m Monitor) (Monitor, error)
	UpdateMonitor(ctx context.Context, id uuid.UUID, in UpdateMonitorInput) (Monitor, error)
	DeleteMonitor(ctx context.Context, id uuid.UUID) error
}
