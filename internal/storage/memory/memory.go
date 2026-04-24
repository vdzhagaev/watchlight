package memory

import (
	"context"
	"slices"
	"sync"

	"github.com/vdzhagaev/watchlight/internal/monitor"

	"github.com/google/uuid"
)

type Storage struct {
	mu       sync.RWMutex
	monitors []monitor.Monitor
}

func New() *Storage {
	return &Storage{
		monitors: monitors,
	}
}

func (s *Storage) Close() error {
	return nil
}

func (s *Storage) GetMonitor(ctx context.Context, id uuid.UUID) (monitor.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.monitors {
		if m.ID == id {
			return m, nil
		}
	}
	return monitor.Monitor{}, monitor.ErrMonitorNotFound
}

func (s *Storage) CreateMonitor(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.monitors = append(s.monitors, m)
	return m, nil
}

func (s *Storage) UpdateMonitor(ctx context.Context, id uuid.UUID, in monitor.UpdateMonitorInput) (monitor.Monitor, error) {
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
	return monitor.Monitor{}, monitor.ErrMonitorNotFound
}

func (s *Storage) GetMonitorList(ctx context.Context) ([]monitor.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.monitors, nil
}

func (s *Storage) DeleteMonitor(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	l := len(s.monitors)
	s.monitors = slices.DeleteFunc(s.monitors, func(m monitor.Monitor) bool {
		return m.ID == id
	})
	if l == len(s.monitors) {
		return monitor.ErrMonitorNotFound
	}
	return nil
}
