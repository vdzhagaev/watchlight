package monitor

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
)

type Service struct {
	repo       Repository
	log        *slog.Logger
	eventsChan chan ConfigChangeEvent
}

func NewService(repo Repository, log *slog.Logger, eventsChan chan ConfigChangeEvent) *Service {
	return &Service{repo: repo, log: log, eventsChan: eventsChan}
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
	after := m.projectJobs()
	svc.syncScheduler(ctx, nil, after)

	return m, nil
}

func (svc *Service) Update(ctx context.Context, id uuid.UUID, in UpdateMonitorInput) error {
	m, err := svc.repo.GetMonitor(ctx, id)
	if err != nil {
		return err
	}
	before := m.projectJobs()

	if in.Host != nil {
		host, err := NewHost(*in.Host)
		if err != nil {
			return err
		}
		m.ChangeHost(host)
		normalized := host.String()
		in.Host = &normalized
	}
	if in.Name != nil {
		m.Rename(*in.Name)
	}

	err = svc.repo.UpdateMonitor(ctx, id, in)
	if err != nil {
		return err
	}
	after := m.projectJobs()
	svc.syncScheduler(ctx, before, after)
	return nil
}

func (svc *Service) Get(ctx context.Context, id uuid.UUID) (Monitor, error) {
	return svc.repo.GetMonitor(ctx, id)
}

func (svc *Service) List(ctx context.Context) ([]Monitor, error) {
	return svc.repo.GetMonitorList(ctx)
}

func (svc *Service) Delete(ctx context.Context, id uuid.UUID) error {
	m, err := svc.repo.GetMonitor(ctx, id)
	if err != nil {
		return err
	}
	err = svc.repo.DeleteMonitor(ctx, id)
	if err != nil {
		return err
	}
	svc.syncScheduler(ctx, m.projectJobs(), nil)
	return nil
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
	before := m.projectJobs()

	hc, err := m.AddHTTPConfig(in)
	if err != nil {
		return HTTPConfig{}, err
	}

	if err := svc.repo.AddHTTPConfig(ctx, hc); err != nil {
		return HTTPConfig{}, err
	}

	after := m.projectJobs()

	svc.syncScheduler(ctx, before, after)

	return hc, nil
}

func (svc *Service) UpdateHTTPCheck(ctx context.Context, monitorID, configID uuid.UUID, in UpdateHTTPConfigInput) error {
	m, err := svc.repo.GetMonitor(ctx, monitorID)
	if err != nil {
		return err
	}

	before := m.projectJobs()

	hc, err := m.UpdateHTTPConfig(configID, in)
	if err != nil {
		return err
	}

	if err := svc.repo.UpdateHTTPConfig(ctx, hc); err != nil {
		return err
	}

	after := m.projectJobs()

	svc.syncScheduler(ctx, before, after)

	return nil
}

func (svc *Service) RemoveHTTPCheck(ctx context.Context, monitorID, configID uuid.UUID) error {
	m, err := svc.repo.GetMonitor(ctx, monitorID)
	if err != nil {
		return err
	}
	before := m.projectJobs()

	if err := m.RemoveHTTPConfig(configID); err != nil {
		return err
	}

	err = svc.repo.RemoveHTTPConfig(ctx, configID)
	if err != nil {
		return err
	}

	after := m.projectJobs()

	svc.syncScheduler(ctx, before, after)
	return nil
}

func (svc *Service) UpdatePing(ctx context.Context, monitorID uuid.UUID, in UpdatePingConfigInput) error {
	m, err := svc.repo.GetMonitor(ctx, monitorID)
	if err != nil {
		return err
	}

	before := m.projectJobs()

	err = m.UpdatePingConfig(in)
	if err != nil {
		return err
	}

	if err := svc.repo.UpdatePingConfig(ctx, m.PingConfig); err != nil {
		return err
	}

	after := m.projectJobs()
	svc.syncScheduler(ctx, before, after)

	return nil
}

func (svc *Service) sendEvent(ctx context.Context, ev ConfigChangeEvent) {
	select {
	case svc.eventsChan <- ev:
	case <-ctx.Done():
		svc.log.Warn("failed to send config event, context cancelled", sl.Err(ctx.Err()))
	}
}

func (svc *Service) syncScheduler(ctx context.Context, before, after map[uuid.UUID]CheckJob) {
	for id, job := range after {
		if _, ok := before[id]; ok && job.Equal(before[id]) {
			svc.sendEvent(ctx, ConfigChangeEvent{Type: EventUpdated, Job: job})
		} else {
			svc.sendEvent(ctx, ConfigChangeEvent{Type: EventCreated, Job: job})
		}
	}
	for id, job := range before {
		if _, ok := after[id]; !ok {
			svc.sendEvent(ctx, ConfigChangeEvent{Type: EventDeleted, Job: job})
		}
	}
}
