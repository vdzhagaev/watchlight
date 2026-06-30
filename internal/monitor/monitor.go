package monitor

import (
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
