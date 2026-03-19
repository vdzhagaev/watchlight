package memory

import (
	"context"
	"sync"

	"vdzhagev/go-uptime-checker/internal/domain"
	"vdzhagev/go-uptime-checker/internal/storage"
)

type Storage struct {
	mu       sync.RWMutex
	lastID   int64
	monitors []domain.Monitor
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

func (s *Storage) GetMonitor(ctx context.Context, id int64) (domain.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, m := range s.monitors {
		if m.ID == id {
			return m, nil
		}
	}
	return domain.Monitor{}, storage.ErrMonitorNotFound
}

func (s *Storage) SaveMonitor(ctx context.Context, m *domain.Monitor) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastID++
	m.ID = s.lastID
	s.monitors = append(monitors, *m)
	return nil
}

func (s *Storage) GetMonitorList(ctx context.Context) ([]domain.Monitor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.monitors, nil
}
