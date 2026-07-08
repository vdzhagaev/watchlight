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

func (svc *Service) Update(ctx context.Context, id uuid.UUID, in UpdateMonitorInput) error {
	if in.Host != nil {
		host, err := NewHost(*in.Host)
		if err != nil {
			return err
		}
		normalized := host.String()
		in.Host = &normalized
	}
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

	return svc.repo.SaveCheckResult(ctx, CheckResult{
		ID:            id,
		MonitorID:     r.MonitorID,
		ConfigID:      r.ConfigID,
		CheckType:     r.CheckType,
		Status:        status,
		StatusCode:    r.StatusCode,
		ResponseTime:  r.ResponseTime,
		CheckedAt:     r.CheckedAt,
		Error:         errorMessage,
		FoundKeywords: r.FoundKeywords,
	})
}

func (svc *Service) AddHTTPCheck(ctx context.Context, monitorID uuid.UUID, in CreateHTTPConfigInput) (HTTPConfig, error) {
	m, err := svc.repo.GetMonitor(ctx, monitorID)
	if err != nil {
		return HTTPConfig{}, err
	}

	hc, err := m.AddHTTPConfig(in)
	if err != nil {
		return HTTPConfig{}, err
	}

	if err := svc.repo.AddHTTPConfig(ctx, hc); err != nil {
		return HTTPConfig{}, err
	}
	return hc, nil
}

func (svc *Service) UpdateHTTPCheck(ctx context.Context, monitorID, configID uuid.UUID, in UpdateHTTPConfigInput) error {
	m, err := svc.repo.GetMonitor(ctx, monitorID)
	if err != nil {
		return err
	}

	hc, err := m.UpdateHTTPConfig(configID, in)
	if err != nil {
		return err
	}

	if err := svc.repo.UpdateHTTPConfig(ctx, hc); err != nil {
		return err
	}
	return nil
}

func (svc *Service) RemoveHTTPCheck(ctx context.Context, monitorID, configID uuid.UUID) error {
	m, err := svc.repo.GetMonitor(ctx, monitorID)
	if err != nil {
		return err
	}
	if err := m.RemoveHTTPConfig(configID); err != nil {
		return err
	}
	return svc.repo.RemoveHTTPConfig(ctx, configID)
}

func (svc *Service) UpdatePing(ctx context.Context, monitorID uuid.UUID, in UpdatePingConfigInput) error {
	m, err := svc.repo.GetMonitor(ctx, monitorID)
	if err != nil {
		return err
	}

	err = m.UpdatePingConfig(in)
	if err != nil {
		return err
	}

	if err := svc.repo.UpdatePingConfig(ctx, m.PingConfig); err != nil {
		return err
	}
	return nil
}
