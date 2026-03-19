package domain

import "time"

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
	ID                int       `json:"id"`
	MonitorID         int       `json:"monitor_id"`
	CheckType         CheckType `json:"check_type"`
	IsEnabled         bool      `json:"is_enabled"`
	CheckInterval     int       `json:"check_interval"`
	CheckTimeout      int       `json:"check_timeout"`
	MaxAttempts       int       `json:"max_attempts"`
	DoErrorScreenshot bool      `json:"do_error_screenshot"`
	Keywords          []string  `json:"keywords,omitempty"`
}

type Monitor struct {
	ID           int64                `json:"id"`
	Name         string               `json:"name"`
	URL          string               `json:"url"`
	Status       MonitorStatus        `json:"status"`
	CheckConfigs []MonitorCheckConfig `json:"checks"`
}

type MonitorCheckResult struct {
	ID             int           `json:"id"`
	MonitorID      int           `json:"monitor_id"`
	ConfigID       int           `json:"config_id"`
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
