package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/monitor"
)

func newIncidentReason(configID uuid.UUID, ct monitor.ConfigType, at time.Time) monitor.Reason {
	resultID, _ := uuid.NewV7()
	return monitor.Reason{
		ConfigID:       configID,
		ConfigType:     ct,
		LastError:      "boom",
		LatestResultID: resultID,
		StartedAt:      at,
		LastSeenAt:     at,
	}
}

// TestStorage_Incident_Lifecycle drives one incident through its whole life
// against a real DB: open -> record a second reason -> recover one (stays open)
// -> recover the last (closes). It is the load-bearing repo test: the recover
// steps catch zombie rows (delete-all correctness) and the final step catches
// close (GetOpenByMonitor must stop returning a resolved incident).
func TestStorage_Incident_Lifecycle(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()

	m := newMonitor(t, "incident.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}

	cfgA, _ := uuid.NewV7()
	cfgB, _ := uuid.NewV7()
	// Truncate to seconds + UTC so the DATETIME round-trip can't drift on
	// sub-second precision or monotonic-clock stripping.
	t0 := time.Now().UTC().Truncate(time.Second)

	// 1. First failure opens the incident; persist it.
	inc, err := monitor.OpenIncident(m.ID, newIncidentReason(cfgA, monitor.PingConfigType, t0))
	if err != nil {
		t.Fatalf("OpenIncident: %v", err)
	}
	if err := st.Save(ctx, inc); err != nil {
		t.Fatalf("Save open: %v", err)
	}

	// 2. Reload: found, exactly one reason under cfgA, fields reconstructed.
	got, found, err := st.GetOpenByMonitor(ctx, m.ID)
	if err != nil {
		t.Fatalf("GetOpenByMonitor: %v", err)
	}
	if !found {
		t.Fatal("open incident must be found")
	}
	if got.ID != inc.ID {
		t.Errorf("incident id = %s, want %s", got.ID, inc.ID)
	}
	if got.MonitorID != m.ID {
		t.Errorf("monitor id = %s, want %s", got.MonitorID, m.ID)
	}
	if !got.StartedAt.Equal(t0) {
		t.Errorf("incident startedAt = %s, want %s", got.StartedAt, t0)
	}
	if got.ResolvedAt != nil {
		t.Errorf("open incident resolvedAt = %v, want nil", got.ResolvedAt)
	}
	if len(got.Reasons) != 1 {
		t.Fatalf("reasons = %d, want 1", len(got.Reasons))
	}
	rA, ok := got.Reasons[cfgA]
	if !ok {
		t.Fatal("reason for cfgA missing after reload")
	}
	if rA.ConfigType != monitor.PingConfigType {
		t.Errorf("cfgA configType = %q, want %q", rA.ConfigType, monitor.PingConfigType)
	}
	if !rA.StartedAt.Equal(t0) {
		t.Errorf("cfgA reason startedAt = %s, want %s", rA.StartedAt, t0)
	}

	// 3. A second config fails: record it and persist. Reload sees both reasons.
	got.RecordFailure(newIncidentReason(cfgB, monitor.HTTPConfigType, t0.Add(time.Minute)))
	if err := st.Save(ctx, got); err != nil {
		t.Fatalf("Save record: %v", err)
	}
	got, found, err = st.GetOpenByMonitor(ctx, m.ID)
	if err != nil || !found {
		t.Fatalf("reload after record: found=%v err=%v", found, err)
	}
	if len(got.Reasons) != 2 {
		t.Fatalf("reasons = %d, want 2", len(got.Reasons))
	}

	// 4. Recover cfgA (not the last): incident stays open, cfgA's row must be
	// gone from the DB (zombie check), cfgB's row must remain.
	closed, err := got.RecoverReason(cfgA, t0.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("RecoverReason cfgA: %v", err)
	}
	if closed {
		t.Fatal("recovering one of two reasons must not close the incident")
	}
	if err := st.Save(ctx, got); err != nil {
		t.Fatalf("Save after recover cfgA: %v", err)
	}
	got, found, err = st.GetOpenByMonitor(ctx, m.ID)
	if err != nil || !found {
		t.Fatalf("reload after recover cfgA: found=%v err=%v", found, err)
	}
	if len(got.Reasons) != 1 {
		t.Fatalf("reasons = %d, want 1 (recovered row still present => zombie)", len(got.Reasons))
	}
	if _, ok := got.Reasons[cfgA]; ok {
		t.Error("cfgA reason must be gone from DB after recover")
	}
	if _, ok := got.Reasons[cfgB]; !ok {
		t.Error("cfgB reason must remain")
	}

	// 5. Recover cfgB (the last one): the incident closes. GetOpenByMonitor must
	// no longer return it, since it filters on resolved_at IS NULL.
	closed, err = got.RecoverReason(cfgB, t0.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("RecoverReason cfgB: %v", err)
	}
	if !closed {
		t.Fatal("recovering the last reason must close the incident")
	}
	if err := st.Save(ctx, got); err != nil {
		t.Fatalf("Save close: %v", err)
	}
	_, found, err = st.GetOpenByMonitor(ctx, m.ID)
	if err != nil {
		t.Fatalf("reload after close: %v", err)
	}
	if found {
		t.Error("closed incident must not be returned by GetOpenByMonitor")
	}
}

// TestStorage_Incident_OneOpenPerMonitor is the DB backstop for the invariant
// "at most one open incident per monitor". The single-writer service is meant
// to enforce this in the domain; the partial unique index idx_incidents_one_open
// is defense in depth, and this test proves it actually fires.
func TestStorage_Incident_OneOpenPerMonitor(t *testing.T) {
	st := newStorage(t)
	ctx := context.Background()

	m := newMonitor(t, "oneopen.example")
	if err := st.CreateMonitor(ctx, m); err != nil {
		t.Fatalf("CreateMonitor: %v", err)
	}
	at := time.Now().UTC().Truncate(time.Second)

	cfg1, _ := uuid.NewV7()
	first, err := monitor.OpenIncident(m.ID, newIncidentReason(cfg1, monitor.PingConfigType, at))
	if err != nil {
		t.Fatalf("OpenIncident first: %v", err)
	}
	if err := st.Save(ctx, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	// A second, distinct incident (fresh id) for the same monitor while the
	// first is still open must be rejected by the partial unique index.
	cfg2, _ := uuid.NewV7()
	second, err := monitor.OpenIncident(m.ID, newIncidentReason(cfg2, monitor.HTTPConfigType, at))
	if err != nil {
		t.Fatalf("OpenIncident second: %v", err)
	}
	if err := st.Save(ctx, second); err == nil {
		t.Error("saving a second open incident for the same monitor must fail")
	}
}
