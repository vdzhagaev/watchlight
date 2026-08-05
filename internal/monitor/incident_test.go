package monitor_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

func newReason(configID uuid.UUID, at time.Time) monitor.Reason {
	resultID, _ := uuid.NewV7()
	return monitor.Reason{
		ConfigID:       configID,
		LastError:      "something happened",
		LatestResultID: resultID,
		StartedAt:      at,
		LastSeenAt:     at,
	}
}

func TestIncident_OpenIncident(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	configID, _ := uuid.NewV7()
	at := time.Now()
	reason := newReason(configID, at)

	inc, err := monitor.OpenIncident(monitorID, reason)
	if err != nil {
		t.Fatalf("error while opening incident: %v", err)
	}

	if inc.ID == uuid.Nil {
		t.Error("id was not generated")
	}

	if inc.MonitorID != monitorID {
		t.Errorf("monitor id = %s, want %s", inc.MonitorID, monitorID)
	}

	if inc.StartedAt != at {
		t.Errorf("started at = %s, want %s", inc.StartedAt, at)
	}

	if len(inc.Reasons) != 1 {
		t.Fatalf("reasons count is %d, want %d", len(inc.Reasons), 1)
	}

	got, ok := inc.Reasons[configID]
	if !ok {
		t.Fatalf("reason not stored under configID key")
	}
	if got != reason {
		t.Errorf("stored reason = %+v, want %+v", got, reason)
	}

	if inc.ResolvedAt != nil {
		t.Errorf("new incident must be open, got resolvedAt %v", inc.ResolvedAt)
	}
	if inc.ResolvedReason != "" {
		t.Errorf("new incident must have no resolved reason, got %q", inc.ResolvedReason)
	}
}

func TestIncident_RecordFailure_NewConfig(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	cfgA, _ := uuid.NewV7()
	cfgB, _ := uuid.NewV7()
	at := time.Now()

	inc, _ := monitor.OpenIncident(monitorID, newReason(cfgA, at))
	inc.RecordFailure(newReason(cfgB, at.Add(time.Minute)))

	if len(inc.Reasons) != 2 {
		t.Fatalf("reasons count = %d, want 2", len(inc.Reasons))
	}
	if _, ok := inc.Reasons[cfgA]; !ok {
		t.Error("reason for cfgA is missing")
	}
	if _, ok := inc.Reasons[cfgB]; !ok {
		t.Error("reason for cfgB is missing")
	}
	if inc.ResolvedAt != nil {
		t.Errorf("recording a failure must not close the incident, got resolvedAt %v", inc.ResolvedAt)
	}
}

func TestIncident_RecordFailure_SameConfig(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	cfg, _ := uuid.NewV7()
	at1 := time.Now()
	at2 := at1.Add(time.Minute)

	inc, _ := monitor.OpenIncident(monitorID, newReason(cfg, at1))

	resultID, _ := uuid.NewV7()
	inc.RecordFailure(monitor.Reason{
		ConfigID:       cfg,
		LastError:      "second failure",
		LatestResultID: resultID,
		StartedAt:      at2,
		LastSeenAt:     at2,
	})

	if len(inc.Reasons) != 1 {
		t.Fatalf("reasons count = %d, want 1 (same config must not add a row)", len(inc.Reasons))
	}

	got := inc.Reasons[cfg]
	// StartedAt is preserved from the first failure; everything else updates.
	if !got.StartedAt.Equal(at1) {
		t.Errorf("startedAt = %s, want %s (must be preserved from first failure)", got.StartedAt, at1)
	}
	if !got.LastSeenAt.Equal(at2) {
		t.Errorf("lastSeenAt = %s, want %s", got.LastSeenAt, at2)
	}
	if got.LastError != "second failure" {
		t.Errorf("lastError = %q, want %q", got.LastError, "second failure")
	}
	if got.LatestResultID != resultID {
		t.Errorf("latestResultID = %s, want %s", got.LatestResultID, resultID)
	}
}

func TestIncident_RecoverReason_NotLast(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	cfgA, _ := uuid.NewV7()
	cfgB, _ := uuid.NewV7()
	at := time.Now()

	inc, _ := monitor.OpenIncident(monitorID, newReason(cfgA, at))
	inc.RecordFailure(newReason(cfgB, at))

	closed, err := inc.RecoverReason(cfgA, at.Add(time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed {
		t.Error("recovering one of two reasons must not close the incident")
	}
	if _, ok := inc.Reasons[cfgA]; ok {
		t.Error("recovered reason must be removed")
	}
	if _, ok := inc.Reasons[cfgB]; !ok {
		t.Error("remaining reason must stay")
	}
	if inc.ResolvedAt != nil {
		t.Errorf("incident must stay open, got resolvedAt %v", inc.ResolvedAt)
	}
}

func TestIncident_RecoverReason_Last(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	cfg, _ := uuid.NewV7()
	at := time.Now()
	resolvedAt := at.Add(time.Minute)

	inc, _ := monitor.OpenIncident(monitorID, newReason(cfg, at))

	closed, err := inc.RecoverReason(cfg, resolvedAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !closed {
		t.Error("recovering the last reason must close the incident")
	}
	if len(inc.Reasons) != 0 {
		t.Errorf("reasons must be empty, got %d", len(inc.Reasons))
	}
	if inc.ResolvedAt == nil {
		t.Fatal("resolvedAt must be set on close")
	}
	if !inc.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("resolvedAt = %s, want %s", inc.ResolvedAt, resolvedAt)
	}
	if inc.ResolvedReason != monitor.ResolveByRecovered {
		t.Errorf("resolvedReason = %q, want %q", inc.ResolvedReason, monitor.ResolveByRecovered)
	}
}

func TestIncident_RecoverReason_Missing(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	cfg, _ := uuid.NewV7()
	missing, _ := uuid.NewV7()
	at := time.Now()

	inc, _ := monitor.OpenIncident(monitorID, newReason(cfg, at))

	closed, err := inc.RecoverReason(missing, at.Add(time.Minute))
	if err == nil {
		t.Error("recovering a missing reason must return an error")
	}
	if closed {
		t.Error("closed must be false on error")
	}
	if len(inc.Reasons) != 1 {
		t.Errorf("reasons must be untouched, got %d", len(inc.Reasons))
	}
	if inc.ResolvedAt != nil {
		t.Errorf("incident must stay open, got resolvedAt %v", inc.ResolvedAt)
	}
}

func TestIncident_RemoveReason_Last(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	cfg, _ := uuid.NewV7()
	at := time.Now()
	resolvedAt := at.Add(time.Minute)

	inc, _ := monitor.OpenIncident(monitorID, newReason(cfg, at))

	closed := inc.RemoveReason(cfg, resolvedAt)
	if !closed {
		t.Error("removing the last reason must close the incident")
	}
	if inc.ResolvedAt == nil {
		t.Fatal("resolvedAt must be set on close")
	}
	if !inc.ResolvedAt.Equal(resolvedAt) {
		t.Errorf("resolvedAt = %s, want %s", inc.ResolvedAt, resolvedAt)
	}
	if inc.ResolvedReason != monitor.ResolveByConfigRemoved {
		t.Errorf("resolvedReason = %q, want %q", inc.ResolvedReason, monitor.ResolveByConfigRemoved)
	}
}

func TestIncident_RemoveReason_NotLast(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	cfgA, _ := uuid.NewV7()
	cfgB, _ := uuid.NewV7()
	at := time.Now()

	inc, _ := monitor.OpenIncident(monitorID, newReason(cfgA, at))
	inc.RecordFailure(newReason(cfgB, at))

	closed := inc.RemoveReason(cfgA, at.Add(time.Minute))
	if closed {
		t.Error("removing one of two reasons must not close the incident")
	}
	if _, ok := inc.Reasons[cfgB]; !ok {
		t.Error("remaining reason must stay")
	}
	if inc.ResolvedAt != nil {
		t.Errorf("incident must stay open, got resolvedAt %v", inc.ResolvedAt)
	}
}

func TestIncident_RemoveReason_Missing(t *testing.T) {
	monitorID, _ := uuid.NewV7()
	cfg, _ := uuid.NewV7()
	missing, _ := uuid.NewV7()
	at := time.Now()

	inc, _ := monitor.OpenIncident(monitorID, newReason(cfg, at))

	closed := inc.RemoveReason(missing, at.Add(time.Minute))
	if closed {
		t.Error("removing an absent reason must not close the incident")
	}
	if len(inc.Reasons) != 1 {
		t.Errorf("reasons must be untouched, got %d", len(inc.Reasons))
	}
	if inc.ResolvedAt != nil {
		t.Errorf("incident must stay open, got resolvedAt %v", inc.ResolvedAt)
	}
}
