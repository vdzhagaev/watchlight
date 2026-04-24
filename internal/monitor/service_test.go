package monitor_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/storage/memory"
)

func newTestService(t *testing.T) *monitor.Service {
	t.Helper()
	return monitor.NewService(memory.New(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func seedMonitor(t *testing.T, svc *monitor.Service) monitor.Monitor {
	t.Helper()
	m, err := svc.Create(context.Background(), monitor.CreateMonitorInput{
		Name: "seed",
		URL:  "https://seed.example",
		CheckConfigs: []monitor.CreateMonitorCheckConfigInput{
			{CheckType: monitor.CheckHTTP, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3},
		},
	})
	if err != nil {
		t.Fatalf("seedMonitor: %v", err)
	}
	return m
}

func TestService_Create_Success(t *testing.T) {
	svc := newTestService(t)
	in := monitor.CreateMonitorInput{
		Name: "Example",
		URL:  "https://example.com",
		CheckConfigs: []monitor.CreateMonitorCheckConfigInput{
			{
				CheckType: monitor.CheckHTTP, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3,
			},
		},
	}

	got, err := svc.Create(context.Background(), in)

	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if got.ID == uuid.Nil {
		t.Error("expected non-nil ID, got nil")
	}

	if got.Status != monitor.MonitorUnknown {
		t.Errorf("expected status %q, got %q", monitor.MonitorUnknown, got.Status)
	}

	if got.Name != in.Name {
		t.Errorf("expected name %q, got %q", in.Name, got.Name)
	}

	if got.URL != in.URL {
		t.Errorf("expected URL %q, got %q", in.URL, got.URL)
	}

	if len(got.CheckConfigs) != 1 {
		t.Fatalf("len(CheckConfigs) = %d, want 1", len(got.CheckConfigs))
	}

	if !got.CheckConfigs[0].IsEnabled {
		t.Error("IsEnabled default should be true")
	}
}

func TestService_Create_Validation(t *testing.T) {
	tests := []struct {
		name    string
		input   monitor.CreateMonitorInput
		wantErr error
	}{
		{
			name:    "empty name",
			input:   monitor.CreateMonitorInput{URL: "http://x.com"},
			wantErr: monitor.ErrMonitorEmptyName,
		},
		{
			name:    "empty url",
			input:   monitor.CreateMonitorInput{Name: "x"},
			wantErr: monitor.ErrMonitorEmptyURL,
		},
		{
			name:    "no check configs",
			input:   monitor.CreateMonitorInput{Name: "x", URL: "http://x.com"},
			wantErr: monitor.ErrMonitorNoChecks,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t)
			_, err := svc.Create(context.Background(), tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_Create_IsEnabledDefault(t *testing.T) {
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
			svc := newTestService(t)

			got, err := svc.Create(context.Background(), monitor.CreateMonitorInput{
				Name: "x",
				URL:  "https://x.com",
				CheckConfigs: []monitor.CreateMonitorCheckConfigInput{
					{
						CheckType:     monitor.CheckHTTP,
						CheckInterval: 60,
						CheckTimeout:  5,
						MaxAttempts:   3,
						IsEnabled:     tt.isEnabled,
					},
				},
			})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			if len(got.CheckConfigs) != 1 {
				t.Fatalf("len(CheckConfigs) = %d, want 1", len(got.CheckConfigs))
			}
			if got.CheckConfigs[0].IsEnabled != tt.want {
				t.Errorf("IsEnabled = %v, want %v", got.CheckConfigs[0].IsEnabled, tt.want)
			}
		})
	}
}

func TestService_Create_CheckConfigValidation(t *testing.T) {
	validInput := func() monitor.CreateMonitorInput {
		return monitor.CreateMonitorInput{
			Name: "x",
			URL:  "https://x.com",
			CheckConfigs: []monitor.CreateMonitorCheckConfigInput{
				{CheckType: monitor.CheckHTTP, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3},
			},
		}
	}

	tests := []struct {
		name    string
		mutate  func(in *monitor.CreateMonitorInput)
		wantErr error
	}{
		{
			name:    "interval below minimum",
			mutate:  func(in *monitor.CreateMonitorInput) { in.CheckConfigs[0].CheckInterval = 5 },
			wantErr: monitor.ErrCheckIntervalTooSmall,
		},
		{
			name:    "timeout below minimum",
			mutate:  func(in *monitor.CreateMonitorInput) { in.CheckConfigs[0].CheckTimeout = 1 },
			wantErr: monitor.ErrCheckTimeoutTooSmall,
		},
		{
			name:    "maxAttempts below minimum",
			mutate:  func(in *monitor.CreateMonitorInput) { in.CheckConfigs[0].MaxAttempts = -1 },
			wantErr: monitor.ErrMaxAttemptsTooSmall,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t)
			in := validInput()
			tt.mutate(&in)
			_, err := svc.Create(context.Background(), in)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_Create_CheckConfigDefaults(t *testing.T) {
	svc := newTestService(t)

	got, err := svc.Create(context.Background(), monitor.CreateMonitorInput{
		Name: "x",
		URL:  "https://x.com",
		CheckConfigs: []monitor.CreateMonitorCheckConfigInput{
			{CheckType: monitor.CheckHTTP}, // все int-поля нулевые
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
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

func TestService_Get_NotFound(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.Get(context.Background(), uuid.New())
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestService_Delete_ThenGet(t *testing.T) {
	svc := newTestService(t)
	m := seedMonitor(t, svc)

	if err := svc.Delete(context.Background(), m.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := svc.Get(context.Background(), m.ID)
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("after delete: err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestService_CreateGet_RoundTrip(t *testing.T) {
	svc := newTestService(t)
	falseVal := false

	in := monitor.CreateMonitorInput{
		Name: "full",
		URL:  "https://full.example",
		CheckConfigs: []monitor.CreateMonitorCheckConfigInput{
			{
				CheckType:         monitor.CheckHTTP,
				IsEnabled:         &falseVal,
				CheckInterval:     120,
				CheckTimeout:      15,
				MaxAttempts:       5,
				DoErrorScreenshot: true,
				Keywords:          []string{"ok", "ready"},
			},
			{
				CheckType:     monitor.CheckPing,
				CheckInterval: 30,
				CheckTimeout:  4,
				MaxAttempts:   2,
			},
		},
	}

	created, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !reflect.DeepEqual(got, created) {
		t.Errorf("round-trip mismatch:\ncreated: %+v\ngot:     %+v", created, got)
	}
}

func TestService_Create_MultipleChecks(t *testing.T) {
	svc := newTestService(t)
	in := monitor.CreateMonitorInput{
		Name: "multi",
		URL:  "https://multi.example",
		CheckConfigs: []monitor.CreateMonitorCheckConfigInput{
			{CheckType: monitor.CheckHTTP, CheckInterval: 60, CheckTimeout: 5, MaxAttempts: 3},
			{CheckType: monitor.CheckPing, CheckInterval: 30, CheckTimeout: 2, MaxAttempts: 2},
			{CheckType: monitor.CheckHeadless, CheckInterval: 300, CheckTimeout: 20, MaxAttempts: 1},
		},
	}

	got, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
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

func TestService_Update_NotFound(t *testing.T) {
	svc := newTestService(t)
	newName := "x"

	_, err := svc.Update(context.Background(), uuid.New(), monitor.UpdateMonitorInput{Name: &newName})
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	svc := newTestService(t)

	err := svc.Delete(context.Background(), uuid.New())
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestService_List_Empty(t *testing.T) {
	svc := newTestService(t)

	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(List) = %d, want 0", len(got))
	}
}

func TestService_Update_PartialKeepsOthers(t *testing.T) {
	svc := newTestService(t)
	m := seedMonitor(t, svc)

	newName := "renamed"
	_, err := svc.Update(context.Background(), m.ID, monitor.UpdateMonitorInput{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := svc.Get(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name != newName {
		t.Errorf("Name = %q, want %q", got.Name, newName)
	}
	if got.URL != m.URL {
		t.Errorf("URL changed: got %q, want %q", got.URL, m.URL)
	}
	if got.Status != m.Status {
		t.Errorf("Status changed: got %q, want %q", got.Status, m.Status)
	}
}
