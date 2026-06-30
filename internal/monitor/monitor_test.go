package monitor_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

// Domain-level tests: they exercise monitor.New() directly — pure construction
// and validation logic, no storage involved.

func TestNew_Success(t *testing.T) {
	in := monitor.CreateMonitorInput{
		Name: "Example",
		Host: "example.com",
		CheckConfigs: []monitor.CreateHTTPConfigInput{
			{CheckType: monitor.CheckHTTP, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3},
		},
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
	if got.URL != in.URL {
		t.Errorf("URL = %q, want %q", got.URL, in.URL)
	}
	if len(got.CheckConfigs) != 1 {
		t.Fatalf("len(CheckConfigs) = %d, want 1", len(got.CheckConfigs))
	}
	if !got.CheckConfigs[0].IsEnabled {
		t.Error("IsEnabled default should be true")
	}
}

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   monitor.CreateMonitorInput
		wantErr error
	}{
		{"empty name", monitor.CreateMonitorInput{URL: "http://x.com"}, monitor.ErrMonitorEmptyName},
		{"empty url", monitor.CreateMonitorInput{Name: "x"}, monitor.ErrMonitorEmptyHost},
		{"no check configs", monitor.CreateMonitorInput{Name: "x", URL: "http://x.com"}, monitor.ErrMonitorNoChecks},
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
			got, err := monitor.New(monitor.CreateMonitorInput{
				Name: "x",
				URL:  "https://x.com",
				CheckConfigs: []monitor.CreateHTTPConfigInput{
					{CheckType: monitor.CheckHTTP, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3, IsEnabled: tt.isEnabled},
				},
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if got.CheckConfigs[0].IsEnabled != tt.want {
				t.Errorf("IsEnabled = %v, want %v", got.CheckConfigs[0].IsEnabled, tt.want)
			}
		})
	}
}

func TestNew_CheckConfigValidation(t *testing.T) {
	valid := func() monitor.CreateMonitorInput {
		return monitor.CreateMonitorInput{
			Name: "x",
			URL:  "https://x.com",
			CheckConfigs: []monitor.CreateHTTPConfigInput{
				{CheckType: monitor.CheckHTTP, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3},
			},
		}
	}
	tests := []struct {
		name    string
		mutate  func(in *monitor.CreateMonitorInput)
		wantErr error
	}{
		{"interval below minimum", func(in *monitor.CreateMonitorInput) { in.CheckConfigs[0].CheckInterval = 5 }, monitor.ErrCheckIntervalTooSmall},
		{"timeout below minimum", func(in *monitor.CreateMonitorInput) { in.CheckConfigs[0].CheckTimeout = 1 }, monitor.ErrCheckTimeoutTooSmall},
		{"maxAttempts below minimum", func(in *monitor.CreateMonitorInput) { in.CheckConfigs[0].MaxAttempts = -1 }, monitor.ErrMaxAttemptsTooSmall},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid()
			tt.mutate(&in)
			_, err := monitor.New(in)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNew_CheckConfigDefaults(t *testing.T) {
	got, err := monitor.New(monitor.CreateMonitorInput{
		Name: "x",
		URL:  "https://x.com",
		CheckConfigs: []monitor.CreateHTTPConfigInput{
			{CheckType: monitor.CheckHTTP}, // all int fields zero -> defaults applied
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cfg := got.CheckConfigs[0]
	if cfg.CheckInterval != monitor.DefaultCheckInterval {
		t.Errorf("CheckInterval = %d, want %d", cfg.CheckInterval, monitor.DefaultCheckInterval)
	}
	if cfg.CheckTimeout != monitor.DefaultCheckTimeout {
		t.Errorf("CheckTimeout = %d, want %d", cfg.CheckTimeout, monitor.DefaultCheckTimeout)
	}
	if cfg.MaxAttempts != monitor.DefaultMaxAttempts {
		t.Errorf("MaxAttempts = %d, want %d", cfg.MaxAttempts, monitor.DefaultMaxAttempts)
	}
}

func TestNew_MultipleChecks(t *testing.T) {
	in := monitor.CreateMonitorInput{
		Name: "multi",
		URL:  "https://multi.example",
		CheckConfigs: []monitor.CreateHTTPConfigInput{
			{CheckType: monitor.CheckHTTP, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3},
			{CheckType: monitor.CheckPing, CheckInterval: 30, CheckTimeout: 2, MaxAttempts: 2},
			{CheckType: monitor.CheckHeadless, CheckInterval: 300, CheckTimeout: 20, MaxAttempts: 1},
		},
	}
	got, err := monitor.New(in)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if len(got.CheckConfigs) != 3 {
		t.Fatalf("len(CheckConfigs) = %d, want 3", len(got.CheckConfigs))
	}

	seen := make(map[uuid.UUID]bool)
	for i, cfg := range got.CheckConfigs {
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
		if cfg.CheckType != in.CheckConfigs[i].CheckType {
			t.Errorf("config[%d].CheckType = %q, want %q (order changed)", i, cfg.CheckType, in.CheckConfigs[i].CheckType)
		}
	}
}
