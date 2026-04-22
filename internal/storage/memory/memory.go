package memory

import (
	"context"
	"slices"
	"sync"

	"vdzhagev/go-uptime-checker/internal/monitor"
	"vdzhagev/go-uptime-checker/internal/storage"
)

type Storage struct {
	mu       sync.RWMutex
	lastID   int64
	monitors []monitor.Monitor
}

func New() *Storage {
	return &Storage{
		lastID:   10,
		monitors: monitors,
	}
}

func (s *Storage) Close() error {
	return nil
}

func (s *Storage) GetMonitor(ctx context.Context, id int64) (monitor.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.monitors {
		if m.ID == id {
			return m, nil
		}
	}
	return monitor.Monitor{}, storage.ErrMonitorNotFound
}

func (s *Storage) SaveMonitor(ctx context.Context, m monitor.CreateMonitorInput) (monitor.Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastID++
	checks := make([]monitor.MonitorCheckConfig, len(m.CheckConfigs))
	for i, chk := range m.CheckConfigs {
		checks[i] = monitor.MonitorCheckConfig{
			ID:                int64(i),
			MonitorID:         s.lastID,
			CheckType:         chk.CheckType,
			IsEnabled:         chk.IsEnabled,
			CheckInterval:     chk.CheckInterval,
			CheckTimeout:      chk.CheckTimeout,
			MaxAttempts:       chk.MaxAttempts,
			DoErrorScreenshot: chk.DoErrorScreenshot,
			Keywords:          chk.Keywords,
		}
	}
	nM := monitor.Monitor{
		ID:           s.lastID,
		URL:          m.URL,
		Name:         m.Name,
		CheckConfigs: checks,
	}
	s.monitors = append(s.monitors, nM)
	return nM, nil
}

func (s *Storage) UpdateMonitor(ctx context.Context, id int64, in monitor.UpdateMonitorInput) (monitor.Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, m := range s.monitors {
		if m.ID == id {
			if in.Name != nil {
				m.Name = *in.Name
			}
			if in.URL != nil {
				m.URL = *in.URL
			}
			if in.Status != nil {
				m.Status = *in.Status
			}
			s.monitors[i] = m
			return m, nil
		}
	}
	return monitor.Monitor{}, storage.ErrMonitorNotFound
}

func (s *Storage) GetMonitorList(ctx context.Context) ([]monitor.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.monitors, nil
}

func (s *Storage) DeleteMonitor(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := len(s.monitors)
	s.monitors = slices.DeleteFunc(s.monitors, func(m monitor.Monitor) bool {
		return m.ID == id
	})
	if l == len(s.monitors) {
		return storage.ErrMonitorNotFound
	}
	return nil
}
