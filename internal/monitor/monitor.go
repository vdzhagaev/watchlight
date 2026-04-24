package monitor

import (
	"time"

	"github.com/google/uuid"
)

type MonitorStatus string

const (
	MonitorUp      MonitorStatus = "up"
	MonitorDown    MonitorStatus = "down"
	MonitorUnknown MonitorStatus = "unknown"
)

type CheckStatus string

const (
	CheckSuccess CheckStatus = "success"
	CheckFailure CheckStatus = "failure"
)

type CheckType string

const (
	CheckPing     CheckType = "ping"
	CheckHTTP     CheckType = "http"
	CheckHeadless CheckType = "headless"
)

type MonitorCheckConfig struct {
	ID                uuid.UUID `json:"id"`
	MonitorID         uuid.UUID `json:"monitor_id"`
	CheckType         CheckType `json:"check_type"`
	IsEnabled         bool      `json:"is_enabled"`
	CheckInterval     int       `json:"check_interval"`
	CheckTimeout      int       `json:"check_timeout"`
	MaxAttempts       int       `json:"max_attempts"`
	DoErrorScreenshot bool      `json:"do_error_screenshot"`
	Keywords          []string  `json:"keywords,omitempty"`
}

type Monitor struct {
	ID           uuid.UUID            `json:"id"`
	Name         string               `json:"name"`
	URL          string               `json:"url"`
	Status       MonitorStatus        `json:"status"`
	CheckConfigs []MonitorCheckConfig `json:"checks"`
}

type MonitorCheckResult struct {
	ID             uuid.UUID     `json:"id"`
	MonitorID      uuid.UUID     `json:"monitor_id"`
	ConfigID       uuid.UUID     `json:"config_id"`
	Status         CheckStatus   `json:"status"`
	StatusCode     int           `json:"status_code"`
	ResponseTime   time.Duration `json:"response_time"`
	CheckedAt      time.Time     `json:"checked_at"`
	Error          string        `json:"error,omitempty"`
	ScreenshotPath string        `json:"screenshot_path,omitempty"`
}

func (m *Monitor) GetConfig(t CheckType) (MonitorCheckConfig, bool) {
	for _, cfg := range m.CheckConfigs {
		if cfg.CheckType == t {
			return cfg, true
		}
	}
	return MonitorCheckConfig{}, false
}

func New(in CreateMonitorInput) (Monitor, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Monitor{}, err
	}
	if in.Name == "" {
		return Monitor{}, ErrMonitorEmptyName
	}

	if in.URL == "" {
		return Monitor{}, ErrMonitorEmptyURL
	}

	if len(in.CheckConfigs) == 0 {
		return Monitor{}, ErrMonitorNoChecks
	}

	configs, err := buildConfigs(id, in.CheckConfigs)
	if err != nil {
		return Monitor{}, err
	}

	return Monitor{
		ID:           id,
		Name:         in.Name,
		URL:          in.URL,
		Status:       MonitorUnknown,
		CheckConfigs: configs,
	}, nil
}

func buildConfigs(id uuid.UUID, configs []CreateMonitorCheckConfigInput) ([]MonitorCheckConfig, error) {
	var checks []MonitorCheckConfig
	for _, chk := range configs {
		checkEnable := true
		if chk.IsEnabled != nil {
			checkEnable = *chk.IsEnabled
		}

		interval := chk.CheckInterval
		if interval == 0 {
			interval = DefaultCheckInterval
		} else if interval < MinCheckInterval {
			return nil, ErrCheckIntervalTooSmall
		}

		timeout := chk.CheckTimeout
		if timeout == 0 {
			timeout = DefaultCheckTimeout
		} else if timeout < MinCheckTimeout {
			return nil, ErrCheckTimeoutTooSmall
		}

		maxAttempts := chk.MaxAttempts
		if maxAttempts == 0 {
			maxAttempts = DefaultMaxAttempts
		} else if maxAttempts < MinMaxAttempts {
			return nil, ErrMaxAttemptsTooSmall
		}

		checkID, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		checks = append(checks, MonitorCheckConfig{
			ID:                checkID,
			MonitorID:         id,
			CheckType:         chk.CheckType,
			IsEnabled:         checkEnable,
			CheckInterval:     interval,
			CheckTimeout:      timeout,
			MaxAttempts:       maxAttempts,
			DoErrorScreenshot: chk.DoErrorScreenshot,
			Keywords:          chk.Keywords,
		})
	}
	return checks, nil
}
