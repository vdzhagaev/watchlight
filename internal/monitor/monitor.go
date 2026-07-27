package monitor

import (
	"slices"

	"github.com/google/uuid"
)

type Monitor struct {
	ID          uuid.UUID
	Name        string
	Host        Host
	Status      MonitorStatus
	PingConfig  PingConfig
	HTTPConfigs []HTTPConfig
}

func New(in CreateMonitorInput) (Monitor, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Monitor{}, err
	}

	err = validateMonitor(in)
	if err != nil {
		return Monitor{}, err
	}

	host, err := NewHost(in.Host)
	if err != nil {
		return Monitor{}, err
	}

	var pingConfig PingConfig

	if in.PingConfig == nil {
		pingConfig = NewDefaultPingConfig(id)
	} else {
		pingConfig, err = NewPingConfig(id, *in.PingConfig)
		if err != nil {
			return Monitor{}, err
		}
	}

	configs, err := buildHTTPConfigs(id, in.HTTPConfigs)
	if err != nil {
		return Monitor{}, err
	}

	return Monitor{
		ID:          id,
		Name:        in.Name,
		Host:        host,
		Status:      MonitorUnknown,
		PingConfig:  pingConfig,
		HTTPConfigs: configs,
	}, nil
}

// ReconstructMonitor rebuilds a Monitor aggregate from trusted stored values and
// its already-reconstructed children. Host is wrapped and Status is cast without
// validation — storage rehydration only.
func ReconstructMonitor(
	id uuid.UUID,
	name, host, status string,
	pingConfig PingConfig,
	httpConfigs []HTTPConfig,
) Monitor {
	return Monitor{
		ID:          id,
		Name:        name,
		Host:        ReconstructHost(host),
		Status:      MonitorStatus(status),
		PingConfig:  pingConfig,
		HTTPConfigs: httpConfigs,
	}
}

func validateMonitor(in CreateMonitorInput) error {
	if in.Name == "" {
		return ErrMonitorEmptyName
	}

	return nil
}

func enabledOrDefault(p *bool) bool {
	if p == nil {
		return true
	}
	return *p
}

func (m *Monitor) ChangeHost(host Host) {
	m.Host = host
}

func (m *Monitor) Rename(name string) {
	m.Name = name
}

func (m *Monitor) ChangeStatus(status MonitorStatus) {
	m.Status = status
}

func (m *Monitor) UpdatePingConfig(in UpdatePingConfigInput) error {
	pingConfig, err := m.PingConfig.Update(in)
	if err != nil {
		return err
	}
	m.PingConfig = pingConfig
	return nil
}

func (m *Monitor) UpdateHTTPConfig(configID uuid.UUID, in UpdateHTTPConfigInput) (HTTPConfig, error) {
	index := slices.IndexFunc(m.HTTPConfigs, func(c HTTPConfig) bool {
		return c.ID == configID
	})
	if index == -1 {
		return HTTPConfig{}, ErrHTTPConfigNotFound
	}

	hc := m.HTTPConfigs[index]

	updated, err := hc.Update(in)
	if err != nil {
		return HTTPConfig{}, err
	}
	dup := slices.IndexFunc(m.HTTPConfigs, func(c HTTPConfig) bool {
		return c.ID != updated.ID && c.Scheme == updated.Scheme && c.Path == updated.Path && c.Method == updated.Method
	})
	if dup != -1 {
		return HTTPConfig{}, ErrHTTPConfigExists
	}
	m.HTTPConfigs[index] = updated
	return updated, nil
}

func (m *Monitor) AddHTTPConfig(in CreateHTTPConfigInput) (HTTPConfig, error) {
	hc, err := NewHTTPConfig(m.ID, in)

	if err != nil {
		return HTTPConfig{}, err
	}

	index := slices.IndexFunc(m.HTTPConfigs, func(c HTTPConfig) bool {
		return c.Scheme == hc.Scheme && c.Path == hc.Path && c.Method == hc.Method
	})

	if index != -1 {
		return HTTPConfig{}, ErrHTTPConfigExists
	}

	m.HTTPConfigs = append(m.HTTPConfigs, hc)
	return hc, nil
}

func (m *Monitor) RemoveHTTPConfig(configID uuid.UUID) error {
	index := slices.IndexFunc(m.HTTPConfigs, func(c HTTPConfig) bool {
		return c.ID == configID
	})
	if index == -1 {
		return ErrHTTPConfigNotFound
	}
	m.HTTPConfigs = slices.Delete(m.HTTPConfigs, index, index+1)
	return nil
}

func (m *Monitor) projectJobs() map[uuid.UUID]CheckJob {
	jobs := make(map[uuid.UUID]CheckJob)
	if m.PingConfig.IsEnabled {
		jobs[m.PingConfig.ID] = m.PingConfig.ToJob(m.Host)
	}
	for _, hc := range m.HTTPConfigs {
		if hc.IsEnabled {
			jobs[hc.ID] = hc.ToJob(m.Host)
		}
	}
	return jobs
}
