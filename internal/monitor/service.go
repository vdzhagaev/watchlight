package monitor

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
	log  *slog.Logger
}

func NewService(repo Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (svc *Service) Create(ctx context.Context, in CreateMonitorInput) (Monitor, error) {
	m, err := New(in)
	if err != nil {
		return Monitor{}, err
	}
	return svc.repo.CreateMonitor(ctx, m)
}

func (svc *Service) Update(ctx context.Context, id uuid.UUID, in UpdateMonitorInput) (Monitor, error) {
	return svc.repo.UpdateMonitor(ctx, id, in)
}

func (svc *Service) Get(ctx context.Context, id uuid.UUID) (Monitor, error) {
	return svc.repo.GetMonitor(ctx, id)
}

func (svc *Service) List(ctx context.Context) ([]Monitor, error) {
	return svc.repo.GetMonitorList(ctx)
}

func (svc *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return svc.repo.DeleteMonitor(ctx, id)
}
