package monitor

import (
	"context"

	"github.com/google/uuid"
)

//go:generate mockery

type Repository interface {
	GetMonitor(ctx context.Context, id uuid.UUID) (Monitor, error)
	GetMonitorList(ctx context.Context) ([]Monitor, error)
	CreateMonitor(ctx context.Context, m Monitor) error
	UpdateMonitor(ctx context.Context, id uuid.UUID, in UpdateMonitorInput) error
	DeleteMonitor(ctx context.Context, id uuid.UUID) error

	SaveCheckResult(ctx context.Context, r CheckResult) error

	// Configs
	UpdatePingConfig(ctx context.Context, cfg PingConfig) error
	UpdateHTTPConfig(ctx context.Context, cfg HTTPConfig) error

	AddHTTPConfig(ctx context.Context, cfg HTTPConfig) error
	RemoveHTTPConfig(ctx context.Context, configID uuid.UUID) error
}
