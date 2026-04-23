package monitor

import (
	"context"
	"log/slog"
)

type Service struct {
	repo Repository
	log  *slog.Logger
}

func NewService(repo Repository, log *slog.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (svc *Service) Create(ctx context.Context, in CreateMonitorInput) (Monitor, error) {
	return svc.repo.SaveMonitor(ctx, in)
}

func (svc *Service) Get(ctx context.Context, id int64) (Monitor, error) {
	return svc.repo.GetMonitor(ctx, id)
}

func (svc *Service) List(ctx context.Context) ([]Monitor, error) {
	return svc.repo.GetMonitorList(ctx)
}

func (svc *Service) Delete(ctx context.Context, id int64) error {
	return svc.repo.DeleteMonitor(ctx, id)
}
