package monitor

import (
	"github.com/google/uuid"
)

const DefaultPingPort uint16 = 443

type PingConfig struct {
	ID          uuid.UUID
	MonitorID   uuid.UUID
	Port        uint16
	IsEnabled   bool
	Interval    Interval
	Timeout     Timeout
	MaxAttempts MaxAttempts
}

type CreatePingConfigInput struct {
	Port        uint16
	IsEnabled   *bool
	Interval    int
	Timeout     int
	MaxAttempts int
}

type UpdatePingConfigInput struct {
	Port        *uint16
	IsEnabled   *bool
	Interval    *int
	Timeout     *int
	MaxAttempts *int
}

func NewPingConfig(monitorID uuid.UUID, inConfig CreatePingConfigInput) (PingConfig, error) {
	port := DefaultPingPort
	if inConfig.Port != 0 {
		port = inConfig.Port
	}
	interval, err := NewInterval(inConfig.Interval)
	if err != nil {
		return PingConfig{}, err
	}
	timeout, err := NewTimeout(inConfig.Timeout)
	if err != nil {
		return PingConfig{}, err
	}
	maxAttempts, err := NewMaxAttempts(inConfig.MaxAttempts)
	if err != nil {
		return PingConfig{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return PingConfig{}, err
	}
	return PingConfig{
		ID:          id,
		MonitorID:   monitorID,
		Port:        port,
		IsEnabled:   enabledOrDefault(inConfig.IsEnabled),
		Interval:    interval,
		Timeout:     timeout,
		MaxAttempts: maxAttempts,
	}, nil
}

func NewDefaultPingConfig(monitorID uuid.UUID) PingConfig {
	pc, _ := NewPingConfig(monitorID, CreatePingConfigInput{
		Port:        0,
		IsEnabled:   nil,
		Interval:    0,
		Timeout:     0,
		MaxAttempts: 0,
	})
	return pc
}

// ReconstructPingConfig rebuilds a PingConfig from trusted, already-validated
// stored values. No validation or defaulting — storage rehydration only.
func ReconstructPingConfig(
	id, monitorID uuid.UUID,
	port uint16,
	isEnabled bool,
	interval, timeout, maxAttempts int,
) PingConfig {
	return PingConfig{
		ID:          id,
		MonitorID:   monitorID,
		Port:        port,
		IsEnabled:   isEnabled,
		Interval:    ReconstructInterval(interval),
		Timeout:     ReconstructTimeout(timeout),
		MaxAttempts: ReconstructMaxAttempts(maxAttempts),
	}
}

func (cfg PingConfig) Update(in UpdatePingConfigInput) (PingConfig, error) {
	port := cfg.Port
	if in.Port != nil {
		if *in.Port == 0 {
			return cfg, ErrInvalidPort
		}
		port = *in.Port
	}

	enabled := cfg.IsEnabled
	if in.IsEnabled != nil {
		enabled = *in.IsEnabled
	}

	interval := cfg.Interval
	if in.Interval != nil {
		i, err := NewInterval(*in.Interval)
		if err != nil {
			return cfg, err
		}
		interval = i
	}

	timeout := cfg.Timeout
	if in.Timeout != nil {
		t, err := NewTimeout(*in.Timeout)
		if err != nil {
			return cfg, err
		}
		timeout = t
	}

	maxAttempts := cfg.MaxAttempts
	if in.MaxAttempts != nil {
		ma, err := NewMaxAttempts(*in.MaxAttempts)
		if err != nil {
			return cfg, err
		}
		maxAttempts = ma
	}

	cfg.Port = port
	cfg.IsEnabled = enabled
	cfg.Interval = interval
	cfg.Timeout = timeout
	cfg.MaxAttempts = maxAttempts
	return cfg, nil
}

func (cfg PingConfig) ToJob(host Host) PingJob {
	return NewPingJob(CreatePingJobInput{
		MonitorID:   cfg.MonitorID,
		ConfigID:    cfg.ID,
		Host:        host,
		Port:        cfg.Port,
		Interval:    cfg.Interval.Duration(),
		Timeout:     cfg.Timeout.Duration(),
		MaxAttempts: cfg.MaxAttempts.Count(),
	})
}
