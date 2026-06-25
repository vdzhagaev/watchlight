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
	err = svc.repo.CreateMonitor(ctx, m)
	if err != nil {
		return Monitor{}, err
	}

	return m, nil
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

func (svc *Service) HandleCheckResult(ctx context.Context, r CheckResultInput) error {
	status := CheckFailure
	if r.Reachable && r.Error == nil {
		status = CheckSuccess
	}
	var errorMessage string
	if r.Error != nil {
		errorMessage = r.Error.Error()
	}

	id, err := uuid.NewV7()
	if err != nil {
		return err
	}

	return svc.repo.SaveCheckResult(ctx, MonitorCheckResult{
		ID:             id,
		MonitorID:      r.MonitorID,
		ConfigID:       r.ConfigID,
		Status:         status,
		StatusCode:     r.StatusCode,
		ResponseTime:   r.ResponseTime,
		CheckedAt:      r.CheckedAt,
		Error:          errorMessage,
		ScreenshotPath: r.ScreenshotPath,
		FoundKeywords:  r.FoundKeywords,
	})
}
