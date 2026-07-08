package monitor_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

// Aggregate-root mutator tests: they exercise the invariant enforcement that
// lives on Monitor (dup key on add/update, not-found on update/remove, port
// validation on ping). Ping is never removable by construction — there is no
// RemovePing method to test.

func ptr[T any](v T) *T { return &v }

// baseMonitor builds a monitor carrying a single https/GET /health check.
func baseMonitor(t *testing.T) monitor.Monitor {
	t.Helper()
	m, err := monitor.New(monitor.CreateMonitorInput{
		Name: "x", Host: "x.com",
		HTTPConfigs: []monitor.CreateHTTPConfigInput{httpConfig()},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return m
}

func TestAddHTTPConfig_Success(t *testing.T) {
	m := baseMonitor(t)

	cfg, err := m.AddHTTPConfig(monitor.CreateHTTPConfigInput{
		Scheme: "https", Path: "/status", Method: "GET",
		Interval: 60, Timeout: 5, MaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("AddHTTPConfig() error = %v", err)
	}
	if cfg.ID == uuid.Nil {
		t.Error("returned config should have a non-nil ID")
	}
	if cfg.MonitorID != m.ID {
		t.Errorf("MonitorID = %v, want %v", cfg.MonitorID, m.ID)
	}
	if len(m.HTTPConfigs) != 2 {
		t.Fatalf("len(HTTPConfigs) = %d, want 2", len(m.HTTPConfigs))
	}
	if last := m.HTTPConfigs[len(m.HTTPConfigs)-1]; last.ID != cfg.ID {
		t.Error("returned config should be the one appended")
	}
}

func TestAddHTTPConfig_DuplicateKey(t *testing.T) {
	m := baseMonitor(t) // https/GET /health

	_, err := m.AddHTTPConfig(monitor.CreateHTTPConfigInput{
		Scheme: "https", Path: "/health", Method: "GET",
		Interval: 60, Timeout: 5, MaxAttempts: 3,
	})
	if !errors.Is(err, monitor.ErrHTTPConfigExists) {
		t.Errorf("err = %v, want ErrHTTPConfigExists", err)
	}
	if len(m.HTTPConfigs) != 1 {
		t.Errorf("duplicate must not be appended; len = %d, want 1", len(m.HTTPConfigs))
	}
}

// The uniqueness key is (scheme, path, method), so the same path under a
// different scheme or method is a distinct check and is allowed.
func TestAddHTTPConfig_SamePathDifferentKey(t *testing.T) {
	tests := []struct {
		name   string
		scheme string
		method string
	}{
		{"different method", "https", "HEAD"},
		{"different scheme", "http", "GET"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := baseMonitor(t) // https/GET /health
			_, err := m.AddHTTPConfig(monitor.CreateHTTPConfigInput{
				Scheme: tt.scheme, Path: "/health", Method: tt.method,
				Interval: 60, Timeout: 5, MaxAttempts: 3,
			})
			if err != nil {
				t.Fatalf("AddHTTPConfig() error = %v, want nil", err)
			}
			if len(m.HTTPConfigs) != 2 {
				t.Errorf("len(HTTPConfigs) = %d, want 2", len(m.HTTPConfigs))
			}
		})
	}
}

func TestAddHTTPConfig_InvalidInput(t *testing.T) {
	m := baseMonitor(t)

	_, err := m.AddHTTPConfig(monitor.CreateHTTPConfigInput{
		Scheme: "ftp", Path: "/x", Method: "GET",
		Interval: 60, Timeout: 5, MaxAttempts: 3,
	})
	if !errors.Is(err, monitor.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
	if len(m.HTTPConfigs) != 1 {
		t.Errorf("invalid config must not be appended; len = %d, want 1", len(m.HTTPConfigs))
	}
}

func TestUpdateHTTPConfig_Success(t *testing.T) {
	m := baseMonitor(t)
	id := m.HTTPConfigs[0].ID

	if _, err := m.UpdateHTTPConfig(id, monitor.UpdateHTTPConfigInput{Interval: ptr(120)}); err != nil {
		t.Fatalf("UpdateHTTPConfig() error = %v", err)
	}
	if got := m.HTTPConfigs[0].Interval.Seconds(); got != 120 {
		t.Errorf("Interval = %d, want 120", got)
	}
	if m.HTTPConfigs[0].ID != id {
		t.Error("update must preserve the config ID")
	}
}

func TestUpdateHTTPConfig_NotFound(t *testing.T) {
	m := baseMonitor(t)

	_, err := m.UpdateHTTPConfig(uuid.New(), monitor.UpdateHTTPConfigInput{Interval: ptr(120)})
	if !errors.Is(err, monitor.ErrHTTPConfigNotFound) {
		t.Errorf("err = %v, want ErrHTTPConfigNotFound", err)
	}
}

// Editing a check's key so it collides with another existing check is rejected,
// and the target must be left unmodified.
func TestUpdateHTTPConfig_CollisionRejected(t *testing.T) {
	m, err := monitor.New(monitor.CreateMonitorInput{
		Name: "x", Host: "x.com",
		HTTPConfigs: []monitor.CreateHTTPConfigInput{
			{Scheme: "https", Path: "/a", Method: "GET", Interval: 60, Timeout: 5, MaxAttempts: 3},
			{Scheme: "https", Path: "/b", Method: "GET", Interval: 60, Timeout: 5, MaxAttempts: 3},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Move /a onto /b, colliding with the second check.
	_, err = m.UpdateHTTPConfig(m.HTTPConfigs[0].ID, monitor.UpdateHTTPConfigInput{Path: ptr("/b")})
	if !errors.Is(err, monitor.ErrHTTPConfigExists) {
		t.Errorf("err = %v, want ErrHTTPConfigExists", err)
	}
	if got := m.HTTPConfigs[0].Path.String(); got != "/a" {
		t.Errorf("target path mutated to %q despite rejection, want /a", got)
	}
}

// A field-level invariant from the child config surfaces through the root.
func TestUpdateHTTPConfig_InvariantPropagates(t *testing.T) {
	m, err := monitor.New(monitor.CreateMonitorInput{
		Name: "x", Host: "x.com",
		HTTPConfigs: []monitor.CreateHTTPConfigInput{
			{Scheme: "https", Path: "/h", Method: "HEAD", Interval: 60, Timeout: 5, MaxAttempts: 3},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = m.UpdateHTTPConfig(m.HTTPConfigs[0].ID, monitor.UpdateHTTPConfigInput{
		Keywords: ptr([]string{"ok"}),
	})
	if !errors.Is(err, monitor.ErrKeywordsRequireGET) {
		t.Errorf("err = %v, want ErrKeywordsRequireGET", err)
	}
}

func TestRemoveHTTPConfig_Success(t *testing.T) {
	m := baseMonitor(t)

	if err := m.RemoveHTTPConfig(m.HTTPConfigs[0].ID); err != nil {
		t.Fatalf("RemoveHTTPConfig() error = %v", err)
	}
	if len(m.HTTPConfigs) != 0 {
		t.Errorf("len(HTTPConfigs) = %d, want 0", len(m.HTTPConfigs))
	}
}

func TestRemoveHTTPConfig_NotFound(t *testing.T) {
	m := baseMonitor(t)

	if err := m.RemoveHTTPConfig(uuid.New()); !errors.Is(err, monitor.ErrHTTPConfigNotFound) {
		t.Errorf("err = %v, want ErrHTTPConfigNotFound", err)
	}
	if len(m.HTTPConfigs) != 1 {
		t.Errorf("nothing should be removed; len = %d, want 1", len(m.HTTPConfigs))
	}
}

func TestUpdatePingConfig_Success(t *testing.T) {
	m := baseMonitor(t)

	err := m.UpdatePingConfig(monitor.UpdatePingConfigInput{
		Port:      ptr(uint16(8443)),
		IsEnabled: ptr(false),
		Interval:  ptr(120),
	})
	if err != nil {
		t.Fatalf("UpdatePingConfig() error = %v", err)
	}
	if m.PingConfig.Port != 8443 {
		t.Errorf("Port = %d, want 8443", m.PingConfig.Port)
	}
	if m.PingConfig.IsEnabled {
		t.Error("ping should be disabled")
	}
	if got := m.PingConfig.Interval.Seconds(); got != 120 {
		t.Errorf("Interval = %d, want 120", got)
	}
}

func TestUpdatePingConfig_InvalidPort(t *testing.T) {
	m := baseMonitor(t)
	orig := m.PingConfig.Port

	err := m.UpdatePingConfig(monitor.UpdatePingConfigInput{Port: ptr(uint16(0))})
	if !errors.Is(err, monitor.ErrInvalidPort) {
		t.Errorf("err = %v, want ErrInvalidPort", err)
	}
	if m.PingConfig.Port != orig {
		t.Errorf("port mutated to %d despite rejection, want %d", m.PingConfig.Port, orig)
	}
}
