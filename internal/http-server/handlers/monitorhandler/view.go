package monitorhandler

import "github.com/vdzhagaev/watchlight/internal/monitor"

// View DTOs are the JSON API representation of the monitor aggregate. They live
// at the transport boundary so the domain (whose value objects keep their fields
// unexported and are not JSON-shaped) can evolve independently of the wire
// contract. Rendering choices — e.g. interval as seconds — belong here, not in
// the domain.

type pingView struct {
	Port        uint16 `json:"port"`
	IsEnabled   bool   `json:"is_enabled"`
	Interval    int    `json:"interval"`
	Timeout     int    `json:"timeout"`
	MaxAttempts int    `json:"max_attempts"`
}

type httpCheckView struct {
	ID          string   `json:"id"`
	Scheme      string   `json:"scheme"`
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	IsEnabled   bool     `json:"is_enabled"`
	Interval    int      `json:"interval"`
	Timeout     int      `json:"timeout"`
	MaxAttempts int      `json:"max_attempts"`
	Keywords    []string `json:"keywords,omitempty"`
}

type monitorView struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Host       string          `json:"host"`
	Status     string          `json:"status"`
	Ping       pingView        `json:"ping"`
	HTTPChecks []httpCheckView `json:"http_checks"`
}

func toMonitorView(m monitor.Monitor) monitorView {
	checks := make([]httpCheckView, 0, len(m.HTTPConfigs))
	for _, c := range m.HTTPConfigs {
		checks = append(checks, httpCheckView{
			ID:          c.ID.String(),
			Scheme:      string(c.Scheme),
			Path:        c.Path.String(),
			Method:      string(c.Method),
			IsEnabled:   c.IsEnabled,
			Interval:    c.Interval.Seconds(),
			Timeout:     c.Timeout.Seconds(),
			MaxAttempts: c.MaxAttempts.Count(),
			Keywords:    c.Keywords,
		})
	}

	return monitorView{
		ID:     m.ID.String(),
		Name:   m.Name,
		Host:   m.Host.String(),
		Status: string(m.Status),
		Ping: pingView{
			Port:        m.PingConfig.Port,
			IsEnabled:   m.PingConfig.IsEnabled,
			Interval:    m.PingConfig.Interval.Seconds(),
			Timeout:     m.PingConfig.Timeout.Seconds(),
			MaxAttempts: m.PingConfig.MaxAttempts.Count(),
		},
		HTTPChecks: checks,
	}
}
