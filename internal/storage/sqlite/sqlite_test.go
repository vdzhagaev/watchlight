package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/storage/sqlite"
)

// Storage-level tests: they verify the real persistence behaviour of the sqlite
// Repository against a throwaway database file (t.TempDir), no mocks.

func newStorage(t *testing.T) *sqlite.Storage {
	t.Helper()
	st, err := sqlite.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func newMonitor(t *testing.T, host string) monitor.Monitor {
	t.Helper()
	m, err := monitor.New(monitor.CreateMonitorInput{
		Name: "seed",
		Host: host,
		HTTPConfigs: []monitor.CreateHTTPConfigInput{
			{Scheme: "https", Path: "/health", Method: "GET", Interval: 60, Timeout: 5, MaxAttempts: 3, Keywords: []string{"ok", "ready"}},
		},
	})
	if err != nil {
		t.Fatalf("build monitor: %v", err)
	}
	return m
}

func TestStorage_CreateGet_RoundTrip(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "seed.example")

	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	got, err := st.GetMonitor(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	if !reflect.DeepEqual(got, m) {
		t.Errorf("round-trip mismatch:\n stored: %+v\n got:    %+v", m, got)
	}
}

func TestStorage_Delete_ThenGet(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "del.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	if err := st.DeleteMonitor(ctx, m.ID); err != nil {
		t.Fatalf("DeleteMonitor: %v", err)
	}
	_, err := st.GetMonitor(ctx, m.ID)
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("after delete: err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestStorage_Update_PartialKeepsOthers(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	m := newMonitor(t, "upd.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	newName := "renamed"
	if err := st.UpdateMonitor(ctx, m.ID, monitor.UpdateMonitorInput{Name: &newName}); err != nil {
		t.Fatalf("UpdateMonitor: %v", err)
	}

	updated, err := st.GetMonitor(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetMonitor: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("Name = %q, want %q", updated.Name, newName)
	}
	if !updated.Host.Equals(m.Host) {
		t.Errorf("Host changed: %q, want %q", updated.Host.String(), m.Host.String())
	}
	if updated.Status != m.Status {
		t.Errorf("Status changed: %q, want %q", updated.Status, m.Status)
	}
	if !reflect.DeepEqual(updated.HTTPConfigs, m.HTTPConfigs) {
		t.Errorf("HTTPConfigs changed:\n want: %+v\n got:  %+v", m.HTTPConfigs, updated.HTTPConfigs)
	}
}

func TestStorage_NotFound(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()
	id := uuid.New()
	name := "x"

	if _, err := st.GetMonitor(ctx, id); !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("Get: err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
	if err := st.UpdateMonitor(ctx, id, monitor.UpdateMonitorInput{Name: &name}); !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("Update: err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
	if err := st.DeleteMonitor(ctx, id); !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("Delete: err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestStorage_List(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()

	got, err := st.GetMonitorList(ctx)
	if err != nil {
		t.Fatalf("GetMonitorList: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty store: len = %d, want 0", len(got))
	}

	if err := st.CreateMonitor(ctx, newMonitor(t, "a.example")); err != nil {
		t.Fatalf("CreateMonitor a: %v", err)
	}
	if err := st.CreateMonitor(ctx, newMonitor(t, "b.example")); err != nil {
		t.Fatalf("CreateMonitor b: %v", err)
	}

	got, err = st.GetMonitorList(ctx)
	if err != nil {
		t.Fatalf("GetMonitorList: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestStorage_Create_DuplicateHost(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()

	if err := st.CreateMonitor(ctx, newMonitor(t, "dup.example")); err != nil {
		t.Fatalf("first CreateMonitor: %v", err)
	}
	// Second monitor: a fresh ID (New generates one) but the same host, which
	// violates the UNIQUE(host) constraint.
	err := st.CreateMonitor(ctx, newMonitor(t, "dup.example"))
	if !errors.Is(err, monitor.ErrMonitorExists) {
		t.Errorf("duplicate host: err = %v, want %v", err, monitor.ErrMonitorExists)
	}
}
