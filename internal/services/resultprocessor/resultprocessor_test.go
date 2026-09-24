package resultprocessor_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/services/resultprocessor"
)

// Processor tests drive HandleCheckResult against hand-rolled fakes for the two
// consumer-defined ports. They cover the two things the old service test owned
// (fresh id + check-status derivation, now in buildCheckResult) plus the real
// meat: the 4-case transition table and the "healthy check of a config that
// isn't part of the incident" guard.

type fakeResultSaver struct {
	saved []monitor.CheckResult
	err   error
}

func (f *fakeResultSaver) SaveCheckResult(_ context.Context, r monitor.CheckResult) error {
	f.saved = append(f.saved, r)
	return f.err
}

type fakeIncidentStore struct {
	inc    monitor.Incident // returned by GetOpenByMonitor
	exists bool
	getErr error

	saved   []monitor.Incident
	saveErr error
}

func (f *fakeIncidentStore) GetOpenByMonitor(_ context.Context, _ uuid.UUID) (monitor.Incident, bool, error) {
	return f.inc, f.exists, f.getErr
}

func (f *fakeIncidentStore) Save(_ context.Context, inc monitor.Incident) error {
	f.saved = append(f.saved, inc)
	return f.saveErr
}

func newProcessor(saver *fakeResultSaver, incidents *fakeIncidentStore) *resultprocessor.ResultProcessor {
	return resultprocessor.New(resultprocessor.Params{
		CheckResultSaver: saver,
		IncidentStore:    incidents,
	})
}

// input builds a CheckResultInput. reachable=true & no error => success.
func input(monitorID, configID uuid.UUID, reachable bool) monitor.CheckResultInput {
	return monitor.CheckResultInput{
		MonitorID: monitorID,
		ConfigID:  configID,
		CheckType: monitor.CheckHTTP,
		Reachable: reachable,
		CheckedAt: time.Now().UTC(),
	}
}

func reason(configID uuid.UUID) monitor.Reason {
	rid, _ := uuid.NewV7()
	now := time.Now().UTC()
	return monitor.Reason{
		ConfigID:       configID,
		ConfigType:     monitor.HTTPConfigType,
		LastError:      "boom",
		LatestResultID: rid,
		StartedAt:      now,
		LastSeenAt:     now,
	}
}

// openIncidentWith builds an open incident whose reasons cover configIDs.
func openIncidentWith(t *testing.T, monitorID uuid.UUID, configIDs ...uuid.UUID) monitor.Incident {
	t.Helper()
	inc, err := monitor.OpenIncident(monitorID, reason(configIDs[0]))
	if err != nil {
		t.Fatalf("OpenIncident: %v", err)
	}
	for _, cfg := range configIDs[1:] {
		inc.RecordFailure(reason(cfg))
	}
	return inc
}

// --- ported from the old Service tests: buildCheckResult behaviour ---

// Every call mints a fresh, non-nil id, so two results from the same input
// persist as distinct rows instead of colliding on UNIQUE(id) (regression #38).
func TestHandleCheckResult_MintsFreshID(t *testing.T) {
	saver := &fakeResultSaver{}
	rp := newProcessor(saver, &fakeIncidentStore{exists: false})
	in := input(uuid.New(), uuid.New(), true)

	for i := 0; i < 2; i++ {
		if err := rp.HandleCheckResult(context.Background(), in); err != nil {
			t.Fatalf("HandleCheckResult #%d: %v", i, err)
		}
	}

	if len(saver.saved) != 2 {
		t.Fatalf("saved %d results, want 2", len(saver.saved))
	}
	if saver.saved[0].ID == uuid.Nil || saver.saved[1].ID == uuid.Nil {
		t.Errorf("minted nil id: %v, %v", saver.saved[0].ID, saver.saved[1].ID)
	}
	if saver.saved[0].ID == saver.saved[1].ID {
		t.Errorf("two results share id %v — would collide on UNIQUE(id)", saver.saved[0].ID)
	}
}

// Check status is derived from raw facts. Failure-without-error
// (Reachable=false, Error=nil) is a legitimate case and must not panic.
func TestHandleCheckResult_DerivesCheckStatus(t *testing.T) {
	tests := []struct {
		name       string
		reachable  bool
		err        error
		wantStatus monitor.CheckStatus
		wantMsg    string
	}{
		{"reachable, no error", true, nil, monitor.CheckSuccess, ""},
		{"unreachable, no error", false, nil, monitor.CheckFailure, ""},
		{"error", false, errors.New("boom"), monitor.CheckFailure, "boom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saver := &fakeResultSaver{}
			rp := newProcessor(saver, &fakeIncidentStore{exists: false})

			in := monitor.CheckResultInput{
				MonitorID: uuid.New(),
				ConfigID:  uuid.New(),
				CheckType: monitor.CheckHTTP,
				Reachable: tt.reachable,
				Error:     tt.err,
				CheckedAt: time.Now().UTC(),
			}
			if err := rp.HandleCheckResult(context.Background(), in); err != nil {
				t.Fatalf("HandleCheckResult: %v", err)
			}

			if len(saver.saved) != 1 {
				t.Fatalf("saved %d results, want 1", len(saver.saved))
			}
			got := saver.saved[0]
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", got.Status, tt.wantStatus)
			}
			if got.Error != tt.wantMsg {
				t.Errorf("Error = %q, want %q", got.Error, tt.wantMsg)
			}
		})
	}
}

// --- transition table: success/failure × open/no incident, plus the guard ---

// success + no open incident => journal only, incident untouched.
func TestHandleCheckResult_SuccessNoIncident_DoesNothing(t *testing.T) {
	saver := &fakeResultSaver{}
	incidents := &fakeIncidentStore{exists: false}
	rp := newProcessor(saver, incidents)

	if err := rp.HandleCheckResult(context.Background(), input(uuid.New(), uuid.New(), true)); err != nil {
		t.Fatalf("HandleCheckResult: %v", err)
	}
	if len(saver.saved) != 1 {
		t.Errorf("result not journaled: saved %d, want 1", len(saver.saved))
	}
	if len(incidents.saved) != 0 {
		t.Errorf("incident saved on a healthy check with no incident")
	}
}

// failure + no open incident => open a new incident carrying the reason.
func TestHandleCheckResult_FailureNoIncident_Opens(t *testing.T) {
	mon, cfg := uuid.New(), uuid.New()
	saver := &fakeResultSaver{}
	incidents := &fakeIncidentStore{exists: false}
	rp := newProcessor(saver, incidents)

	if err := rp.HandleCheckResult(context.Background(), input(mon, cfg, false)); err != nil {
		t.Fatalf("HandleCheckResult: %v", err)
	}
	if len(incidents.saved) != 1 {
		t.Fatalf("incidents saved %d, want 1", len(incidents.saved))
	}
	inc := incidents.saved[0]
	if inc.MonitorID != mon {
		t.Errorf("incident monitorID = %s, want %s", inc.MonitorID, mon)
	}
	if _, ok := inc.Reasons[cfg]; !ok {
		t.Errorf("opened incident missing reason for cfg %s", cfg)
	}
	if inc.ResolvedAt != nil {
		t.Errorf("newly opened incident must be open, resolvedAt = %v", inc.ResolvedAt)
	}
}

// failure + open incident => record the new reason, incident stays open.
func TestHandleCheckResult_FailureOpenIncident_Records(t *testing.T) {
	mon, cfgA, cfgB := uuid.New(), uuid.New(), uuid.New()
	saver := &fakeResultSaver{}
	incidents := &fakeIncidentStore{exists: true, inc: openIncidentWith(t, mon, cfgA)}
	rp := newProcessor(saver, incidents)

	if err := rp.HandleCheckResult(context.Background(), input(mon, cfgB, false)); err != nil {
		t.Fatalf("HandleCheckResult: %v", err)
	}
	if len(incidents.saved) != 1 {
		t.Fatalf("incidents saved %d, want 1", len(incidents.saved))
	}
	inc := incidents.saved[0]
	if _, ok := inc.Reasons[cfgB]; !ok {
		t.Errorf("new reason for cfgB not recorded")
	}
	if _, ok := inc.Reasons[cfgA]; !ok {
		t.Errorf("existing reason for cfgA lost")
	}
	if inc.ResolvedAt != nil {
		t.Errorf("incident must stay open while a config is still failing")
	}
}

// success + open incident, config is one of several reasons => recover it,
// incident stays open because another config is still down.
func TestHandleCheckResult_SuccessOpenIncident_RecoversOne(t *testing.T) {
	mon, cfgA, cfgB := uuid.New(), uuid.New(), uuid.New()
	saver := &fakeResultSaver{}
	incidents := &fakeIncidentStore{exists: true, inc: openIncidentWith(t, mon, cfgA, cfgB)}
	rp := newProcessor(saver, incidents)

	if err := rp.HandleCheckResult(context.Background(), input(mon, cfgA, true)); err != nil {
		t.Fatalf("HandleCheckResult: %v", err)
	}
	if len(incidents.saved) != 1 {
		t.Fatalf("incidents saved %d, want 1", len(incidents.saved))
	}
	inc := incidents.saved[0]
	if _, ok := inc.Reasons[cfgA]; ok {
		t.Errorf("recovered reason for cfgA must be gone")
	}
	if _, ok := inc.Reasons[cfgB]; !ok {
		t.Errorf("cfgB reason must remain")
	}
	if inc.ResolvedAt != nil {
		t.Errorf("incident must stay open (cfgB still down)")
	}
}

// success + open incident, config is the last reason => recover closes it.
func TestHandleCheckResult_SuccessOpenIncident_RecoversLast_Closes(t *testing.T) {
	mon, cfg := uuid.New(), uuid.New()
	saver := &fakeResultSaver{}
	incidents := &fakeIncidentStore{exists: true, inc: openIncidentWith(t, mon, cfg)}
	rp := newProcessor(saver, incidents)

	if err := rp.HandleCheckResult(context.Background(), input(mon, cfg, true)); err != nil {
		t.Fatalf("HandleCheckResult: %v", err)
	}
	if len(incidents.saved) != 1 {
		t.Fatalf("incidents saved %d, want 1", len(incidents.saved))
	}
	inc := incidents.saved[0]
	if len(inc.Reasons) != 0 {
		t.Errorf("reasons must be empty after recovering the last one, got %d", len(inc.Reasons))
	}
	if inc.ResolvedAt == nil {
		t.Errorf("incident must be closed after the last reason recovers")
	}
}

// success + open incident, but for a config that is NOT part of the incident
// (another config is what's down) => guard: journal only, incident untouched.
func TestHandleCheckResult_SuccessOpenIncident_UnrelatedConfig_NoOp(t *testing.T) {
	mon, cfgDown, cfgHealthy := uuid.New(), uuid.New(), uuid.New()
	saver := &fakeResultSaver{}
	incidents := &fakeIncidentStore{exists: true, inc: openIncidentWith(t, mon, cfgDown)}
	rp := newProcessor(saver, incidents)

	if err := rp.HandleCheckResult(context.Background(), input(mon, cfgHealthy, true)); err != nil {
		t.Fatalf("healthy check of an unrelated config errored: %v", err)
	}
	if len(saver.saved) != 1 {
		t.Errorf("result must still be journaled")
	}
	if len(incidents.saved) != 0 {
		t.Errorf("incident must not be saved for a healthy config that isn't a reason")
	}
}

// An unknown check type on the failure path returns an error (not a panic):
// a broken/unwired check kind fails one result, it must not crash the loop.
func TestHandleCheckResult_UnknownCheckType_Errors(t *testing.T) {
	saver := &fakeResultSaver{}
	rp := newProcessor(saver, &fakeIncidentStore{exists: false})

	in := monitor.CheckResultInput{
		MonitorID: uuid.New(),
		ConfigID:  uuid.New(),
		CheckType: monitor.CheckHeadless, // defined but not mapped in buildReason
		Reachable: false,                 // failure branch => buildReason is called
		CheckedAt: time.Now().UTC(),
	}
	if err := rp.HandleCheckResult(context.Background(), in); err == nil {
		t.Error("unknown check type on failure must return an error, not nil")
	}
}
