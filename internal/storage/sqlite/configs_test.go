package sqlite_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

// Config-command round-trip tests: they persist a single ping/http config change
// through the narrow commands and read it back via GetMonitor. This is the only
// layer that exercises serialization (notably keywords JSON) and SQL-level error
// mapping — things the in-memory domain tests cannot see.

func ptr[T any](v T) *T { return &v }

func findHTTPConfig(t *testing.T, cfgs []monitor.HTTPConfig, id uuid.UUID) monitor.HTTPConfig {
	t.Helper()
	for _, c := range cfgs {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("http config %s not found in %d configs", id, len(cfgs))
	return monitor.HTTPConfig{}
}

func TestStorage_AddHTTPConfig_RoundTrip(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "add.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	cfg, err := m.AddHTTPConfig(monitor.CreateHTTPConfigInput{
		Scheme: "https", Path: "/api", Method: "GET",
		Interval: 60, Timeout: 5, MaxAttempts: 3,
		Keywords: []string{"pong"},
	})
	if err != nil {
		t.Fatalf("AddHTTPConfig (domain): %v", err)
	}

	if err := st.AddHTTPConfig(ctx, cfg); err != nil {
		t.Fatalf("AddHTTPConfig (storage): %v", err)
	}

	got, err := st.GetMonitor(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	found := findHTTPConfig(t, got.HTTPConfigs, cfg.ID)
	if !reflect.DeepEqual(found, cfg) {
		t.Errorf("round-trip mismatch:\n stored: %+v\n got:    %+v", cfg, found)
	}
}

func TestStorage_AddHTTPConfig_Duplicate(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "adddup.example") // already carries https/GET /health
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	// NewHTTPConfig skips the aggregate dup-check, so it yields a fresh-id config
	// that collides on UNIQUE(monitor_id, scheme, path, method) at the DB.
	dup, err := monitor.NewHTTPConfig(m.ID, monitor.CreateHTTPConfigInput{
		Scheme: "https", Path: "/health", Method: "GET",
		Interval: 60, Timeout: 5, MaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("NewHTTPConfig: %v", err)
	}

	if err := st.AddHTTPConfig(ctx, dup); !errors.Is(err, monitor.ErrHTTPConfigExists) {
		t.Errorf("duplicate key: err = %v, want %v", err, monitor.ErrHTTPConfigExists)
	}
}

func TestStorage_UpdateHTTPConfig_RoundTrip(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "updhttp.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	id := m.HTTPConfigs[0].ID
	// Change interval and keywords: the keywords swap is the CSV/JSON round-trip guard.
	updated, err := m.UpdateHTTPConfig(id, monitor.UpdateHTTPConfigInput{
		Interval: ptr(120),
		Keywords: ptr([]string{"changed"}),
	})
	if err != nil {
		t.Fatalf("UpdateHTTPConfig (domain): %v", err)
	}

	if err := st.UpdateHTTPConfig(ctx, updated); err != nil {
		t.Fatalf("UpdateHTTPConfig (storage): %v", err)
	}

	got, err := st.GetMonitor(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	found := findHTTPConfig(t, got.HTTPConfigs, id)
	if !reflect.DeepEqual(found, updated) {
		t.Errorf("round-trip mismatch:\n stored: %+v\n got:    %+v", updated, found)
	}
}

func TestStorage_UpdateHTTPConfig_NotFound(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "updnf.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	// A fresh-id config that was never persisted.
	ghost, err := monitor.NewHTTPConfig(m.ID, monitor.CreateHTTPConfigInput{
		Scheme: "https", Path: "/ghost", Method: "GET",
		Interval: 60, Timeout: 5, MaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("NewHTTPConfig: %v", err)
	}

	if err := st.UpdateHTTPConfig(ctx, ghost); !errors.Is(err, monitor.ErrHTTPConfigNotFound) {
		t.Errorf("update missing: err = %v, want %v", err, monitor.ErrHTTPConfigNotFound)
	}
}

func TestStorage_RemoveHTTPConfig_RoundTrip(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "rmhttp.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	id := m.HTTPConfigs[0].ID
	if err := st.RemoveHTTPConfig(ctx, id); err != nil {
		t.Fatalf("RemoveHTTPConfig: %v", err)
	}

	got, err := st.GetMonitor(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	if len(got.HTTPConfigs) != 0 {
		t.Errorf("len(HTTPConfigs) = %d, want 0 after removal", len(got.HTTPConfigs))
	}
}

func TestStorage_RemoveHTTPConfig_NotFound(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "rmnf.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	if err := st.RemoveHTTPConfig(ctx, uuid.New()); !errors.Is(err, monitor.ErrHTTPConfigNotFound) {
		t.Errorf("remove missing: err = %v, want %v", err, monitor.ErrHTTPConfigNotFound)
	}
}

func TestStorage_UpdatePingConfig_RoundTrip(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "updping.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	if err := m.UpdatePingConfig(monitor.UpdatePingConfigInput{
		Port:      ptr(uint16(8443)),
		IsEnabled: ptr(false),
		Interval:  ptr(120),
	}); err != nil {
		t.Fatalf("UpdatePingConfig (domain): %v", err)
	}

	if err := st.UpdatePingConfig(ctx, m.PingConfig); err != nil {
		t.Fatalf("UpdatePingConfig (storage): %v", err)
	}

	got, err := st.GetMonitor(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	if !reflect.DeepEqual(got.PingConfig, m.PingConfig) {
		t.Errorf("ping round-trip mismatch:\n stored: %+v\n got:    %+v", m.PingConfig, got.PingConfig)
	}
}

func TestStorage_UpdatePingConfig_NotFound(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "pingnf.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	// A ping config with a fresh id that maps to no stored row.
	ghost, err := monitor.NewPingConfig(m.ID, monitor.CreatePingConfigInput{
		Port: 443, Interval: 60, Timeout: 5, MaxAttempts: 3,
	})
	if err != nil {
		t.Fatalf("NewPingConfig: %v", err)
	}

	if err := st.UpdatePingConfig(ctx, ghost); !errors.Is(err, monitor.ErrPingConfigNotFound) {
		t.Errorf("update missing ping: err = %v, want %v", err, monitor.ErrPingConfigNotFound)
	}
}
