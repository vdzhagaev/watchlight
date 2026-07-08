package monitor_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

// Domain-level tests: they exercise monitor.New() directly — pure construction
// and validation logic, no storage involved.

func httpConfig() monitor.CreateHTTPConfigInput {
	return monitor.CreateHTTPConfigInput{
		Scheme: "https", Path: "/health", Method: "GET",
		Interval: 60, Timeout: 5, MaxAttempts: 3,
	}
}

func TestNew_Success(t *testing.T) {
	in := monitor.CreateMonitorInput{
		Name:        "Example",
		Host:        "example.com",
		HTTPConfigs: []monitor.CreateHTTPConfigInput{httpConfig()},
	}

	got, err := monitor.New(in)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if got.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if got.Status != monitor.MonitorUnknown {
		t.Errorf("Status = %q, want %q", got.Status, monitor.MonitorUnknown)
	}
	if got.Name != in.Name {
		t.Errorf("Name = %q, want %q", got.Name, in.Name)
	}
	if got.Host.String() != in.Host {
		t.Errorf("Host = %q, want %q", got.Host.String(), in.Host)
	}
	if len(got.HTTPConfigs) != 1 {
		t.Fatalf("len(HTTPConfigs) = %d, want 1", len(got.HTTPConfigs))
	}
	if !got.HTTPConfigs[0].IsEnabled {
		t.Error("IsEnabled default should be true")
	}
}

// Every monitor gets an intrinsic ping config even when none is supplied.
func TestNew_DefaultPingConfig(t *testing.T) {
	got, err := monitor.New(monitor.CreateMonitorInput{Name: "x", Host: "x.com"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got.PingConfig.ID == uuid.Nil {
		t.Error("default ping config should have a non-nil ID")
	}
	if got.PingConfig.MonitorID != got.ID {
		t.Errorf("ping MonitorID = %v, want %v", got.PingConfig.MonitorID, got.ID)
	}
	if got.PingConfig.Port != monitor.DefaultPingPort {
		t.Errorf("ping Port = %d, want %d", got.PingConfig.Port, monitor.DefaultPingPort)
	}
	if !got.PingConfig.IsEnabled {
		t.Error("default ping should be enabled")
	}
}

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   monitor.CreateMonitorInput
		wantErr error
	}{
		{"empty name", monitor.CreateMonitorInput{Host: "x.com"}, monitor.ErrMonitorEmptyName},
		{"empty host", monitor.CreateMonitorInput{Name: "x"}, monitor.ErrMonitorEmptyHost},
		{"invalid host", monitor.CreateMonitorInput{Name: "x", Host: "http://x.com/y"}, monitor.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := monitor.New(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNew_IsEnabledDefault(t *testing.T) {
	trueVal, falseVal := true, false
	tests := []struct {
		name      string
		isEnabled *bool
		want      bool
	}{
		{"nil defaults to true", nil, true},
		{"explicit true stays true", &trueVal, true},
		{"explicit false stays false", &falseVal, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := httpConfig()
			cfg.IsEnabled = tt.isEnabled
			got, err := monitor.New(monitor.CreateMonitorInput{
				Name: "x", Host: "x.com",
				HTTPConfigs: []monitor.CreateHTTPConfigInput{cfg},
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if got.HTTPConfigs[0].IsEnabled != tt.want {
				t.Errorf("IsEnabled = %v, want %v", got.HTTPConfigs[0].IsEnabled, tt.want)
			}
		})
	}
}

func TestNew_HTTPConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *monitor.CreateHTTPConfigInput)
		wantErr error
	}{
		{"interval below minimum", func(c *monitor.CreateHTTPConfigInput) { c.Interval = 5 }, monitor.ErrCheckIntervalTooSmall},
		{"timeout below minimum", func(c *monitor.CreateHTTPConfigInput) { c.Timeout = 1 }, monitor.ErrCheckTimeoutTooSmall},
		{"maxAttempts below minimum", func(c *monitor.CreateHTTPConfigInput) { c.MaxAttempts = -1 }, monitor.ErrMaxAttemptsTooSmall},
		{"invalid scheme", func(c *monitor.CreateHTTPConfigInput) { c.Scheme = "ftp" }, monitor.ErrValidation},
		{"invalid method", func(c *monitor.CreateHTTPConfigInput) { c.Method = "PUT" }, monitor.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := httpConfig()
			tt.mutate(&cfg)
			_, err := monitor.New(monitor.CreateMonitorInput{
				Name: "x", Host: "x.com",
				HTTPConfigs: []monitor.CreateHTTPConfigInput{cfg},
			})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNew_HTTPConfigDefaults(t *testing.T) {
	got, err := monitor.New(monitor.CreateMonitorInput{
		Name: "x", Host: "x.com",
		HTTPConfigs: []monitor.CreateHTTPConfigInput{
			{Scheme: "https", Method: "GET"}, // int fields zero -> defaults applied
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cfg := got.HTTPConfigs[0]
	if cfg.Interval.Seconds() != monitor.DefaultCheckInterval {
		t.Errorf("Interval = %d, want %d", cfg.Interval.Seconds(), monitor.DefaultCheckInterval)
	}
	if cfg.Timeout.Seconds() != monitor.DefaultCheckTimeout {
		t.Errorf("Timeout = %d, want %d", cfg.Timeout.Seconds(), monitor.DefaultCheckTimeout)
	}
	if cfg.MaxAttempts.Count() != monitor.DefaultMaxAttempts {
		t.Errorf("MaxAttempts = %d, want %d", cfg.MaxAttempts.Count(), monitor.DefaultMaxAttempts)
	}
	// Path defaults to "/" when omitted.
	if cfg.Path.String() != "/" {
		t.Errorf("Path = %q, want %q", cfg.Path.String(), "/")
	}
}

func TestNew_MultipleHTTPChecks(t *testing.T) {
	in := monitor.CreateMonitorInput{
		Name: "multi", Host: "multi.example",
		HTTPConfigs: []monitor.CreateHTTPConfigInput{
			{Scheme: "https", Path: "/a", Method: "GET", Interval: 60, Timeout: 5, MaxAttempts: 3},
			{Scheme: "https", Path: "/b", Method: "HEAD", Interval: 30, Timeout: 2, MaxAttempts: 2},
			{Scheme: "http", Path: "/c", Method: "GET", Interval: 300, Timeout: 20, MaxAttempts: 1},
		},
	}
	got, err := monitor.New(in)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(got.HTTPConfigs) != 3 {
		t.Fatalf("len(HTTPConfigs) = %d, want 3", len(got.HTTPConfigs))
	}

	seen := make(map[uuid.UUID]bool)
	for i, cfg := range got.HTTPConfigs {
		if cfg.ID == uuid.Nil {
			t.Errorf("config[%d].ID is nil", i)
		}
		if seen[cfg.ID] {
			t.Errorf("config[%d].ID = %v duplicates earlier config", i, cfg.ID)
		}
		seen[cfg.ID] = true

		if cfg.MonitorID != got.ID {
			t.Errorf("config[%d].MonitorID = %v, want %v", i, cfg.MonitorID, got.ID)
		}
		if cfg.Path.String() != in.HTTPConfigs[i].Path {
			t.Errorf("config[%d].Path = %q, want %q (order changed)", i, cfg.Path.String(), in.HTTPConfigs[i].Path)
		}
	}
}
