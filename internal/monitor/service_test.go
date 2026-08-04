package monitor_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/monitor/mocks"
)

// Service-level tests: they verify the Service's own behaviour — that it
// delegates to the Repository and propagates results/errors. The store is a
// generated mock, so we script exactly what the repository returns.

func newSvc(t *testing.T) (*monitor.Service, *mocks.MockRepository, chan monitor.ConfigChangeEvent) {
	t.Helper()
	repo := mocks.NewMockRepository(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	eventsChan := make(chan monitor.ConfigChangeEvent, 16)
	return monitor.NewService(repo, log, eventsChan), repo, eventsChan
}

func validInput() monitor.CreateMonitorInput {
	return monitor.CreateMonitorInput{
		Name: "x",
		Host: "x.com",
		HTTPConfigs: []monitor.CreateHTTPConfigInput{
			{Scheme: "https", Path: "/", Method: "GET", Interval: 60, Timeout: 5, MaxAttempts: 3},
		},
	}
}

// Create builds the monitor via New() and forwards it to the repository.
func TestService_Create_DelegatesToRepo(t *testing.T) {
	svc, repo, _ := newSvc(t)

	repo.EXPECT().
		CreateMonitor(mock.Anything, mock.AnythingOfType("monitor.Monitor")).
		Return(nil).
		Once()

	got, err := svc.Create(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if got.Name != "x" {
		t.Errorf("Name = %q, want %q", got.Name, "x")
	}
}

// An error from the repository is propagated unchanged.
func TestService_Create_PropagatesRepoError(t *testing.T) {
	svc, repo, _ := newSvc(t)

	repo.EXPECT().
		CreateMonitor(mock.Anything, mock.Anything).
		Return(monitor.ErrMonitorExists).
		Once()

	_, err := svc.Create(context.Background(), validInput())
	if !errors.Is(err, monitor.ErrMonitorExists) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorExists)
	}
}

// Invalid input must fail in New() *before* the repository is touched. We set
// no expectations on the mock: if Create called the repo, the generated mock
// would panic ("no return value specified for CreateMonitor") and fail the test.
func TestService_Create_InvalidInput_SkipsRepo(t *testing.T) {
	svc, _, _ := newSvc(t)

	_, err := svc.Create(context.Background(), monitor.CreateMonitorInput{})
	if !errors.Is(err, monitor.ErrMonitorEmptyName) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorEmptyName)
	}
}

func TestService_Get_Propagates(t *testing.T) {
	svc, repo, _ := newSvc(t)
	id := uuid.New()

	repo.EXPECT().GetMonitor(mock.Anything, id).
		Return(monitor.Monitor{}, monitor.ErrMonitorNotFound).
		Once()

	_, err := svc.Get(context.Background(), id)
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestService_Update_Propagates(t *testing.T) {
	svc, repo, _ := newSvc(t)
	id := uuid.New()
	name := "new"

	repo.EXPECT().GetMonitor(mock.Anything, id).
		Return(monitor.Monitor{}, nil).
		Once()
	repo.EXPECT().
		UpdateMonitor(mock.Anything, id, mock.AnythingOfType("monitor.UpdateMonitorInput")).
		Return(monitor.ErrMonitorNotFound).
		Once()

	_, err := svc.Update(context.Background(), id, monitor.UpdateMonitorInput{Name: &name})
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestService_Update_NameOnly_NoEvents(t *testing.T) {
	svc, repo, events := newSvc(t)
	m, _ := monitor.New(validInput())
	name := "new"

	repo.EXPECT().GetMonitor(mock.Anything, m.ID).
		Return(m, nil).
		Once()
	repo.EXPECT().
		UpdateMonitor(mock.Anything, m.ID, mock.AnythingOfType("monitor.UpdateMonitorInput")).
		Return(nil).
		Once()

	updatedMonitor, err := svc.Update(context.Background(), m.ID, monitor.UpdateMonitorInput{Name: &name})

	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updatedMonitor.Name != name {
		t.Errorf("name = %s, want %s", updatedMonitor.Name, name)
	}

	select {
	case ev := <-events:
		t.Errorf("expected no events, got %+v", ev)
	default:
	}
}

func TestService_Update_HostOnly_EmitsUpdated(t *testing.T) {
	svc, repo, events := newSvc(t)
	m, _ := monitor.New(validInput())
	newHost := "google.com"

	repo.EXPECT().GetMonitor(mock.Anything, m.ID).
		Return(m, nil).
		Once()
	repo.EXPECT().
		UpdateMonitor(mock.Anything, m.ID, mock.AnythingOfType("monitor.UpdateMonitorInput")).
		Return(nil).
		Once()

	updatedMonitor, err := svc.Update(context.Background(), m.ID, monitor.UpdateMonitorInput{Host: &newHost})

	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updatedMonitor.Host.String() != newHost {
		t.Errorf("host = %v, want %s", updatedMonitor.Host, newHost)
	}

	n := len(events)

	neededN := 0
	if m.PingConfig.IsEnabled {
		neededN += 1
	}
	for _, cfg := range m.HTTPConfigs {
		if cfg.IsEnabled {
			neededN += 1
		}
	}

	if n != neededN {
		t.Fatalf("want %d events (ping + http), got %d", neededN, n)
	}

	for range n {
		ev := <-events
		if ev.Type != monitor.EventUpdated {
			t.Errorf("type = %v, want EventUpdated", ev.Type)
		}
		if !strings.Contains(ev.Job.Target(), newHost) {
			t.Errorf("target = %q, want to contain %q", ev.Job.Target(), newHost)
		}
	}
}

func TestService_AddHTTPCheck(t *testing.T) {
	// expectAdd scripts the happy-path repo calls: GetMonitor (always) plus a
	// successful AddHTTPConfig. Error rows that fail domain validation before
	// the repo write use expectGet instead — declaring AddHTTPConfig here would
	// make mockery fail on "expected but not called".
	expectAdd := func(repo *mocks.MockRepository, m monitor.Monitor) {
		repo.EXPECT().GetMonitor(mock.Anything, m.ID).Return(m, nil).Once()
		repo.EXPECT().AddHTTPConfig(mock.Anything, mock.AnythingOfType("monitor.HTTPConfig")).
			Return(nil).Once()
	}
	expectGet := func(repo *mocks.MockRepository, m monitor.Monitor) {
		repo.EXPECT().GetMonitor(mock.Anything, m.ID).Return(m, nil).Once()
	}

	tests := []struct {
		name       string
		input      monitor.CreateHTTPConfigInput
		setup      func(repo *mocks.MockRepository, m monitor.Monitor)
		wantErr    error
		wantEvents int
		// check runs row-specific assertions on the returned config (nil to skip).
		check func(t *testing.T, hc monitor.HTTPConfig)
	}{
		{
			// zero Interval/Timeout/MaxAttempts fall back to the domain defaults.
			name:       "default values",
			input:      monitor.CreateHTTPConfigInput{Scheme: "https", Path: "/healthcheck", Method: "GET"},
			setup:      expectAdd,
			wantEvents: 1,
			check: func(t *testing.T, hc monitor.HTTPConfig) {
				if !hc.IsEnabled {
					t.Errorf("enabled = false, want true")
				}
				if hc.Interval.Seconds() != monitor.DefaultCheckInterval {
					t.Errorf("interval = %d, want %d", hc.Interval.Seconds(), monitor.DefaultCheckInterval)
				}
				if hc.Timeout.Seconds() != monitor.DefaultCheckTimeout {
					t.Errorf("timeout = %d, want %d", hc.Timeout.Seconds(), monitor.DefaultCheckTimeout)
				}
				if hc.MaxAttempts.Count() != monitor.DefaultMaxAttempts {
					t.Errorf("maxAttempts = %d, want %d", hc.MaxAttempts.Count(), monitor.DefaultMaxAttempts)
				}
				if len(hc.Keywords) != 0 {
					t.Errorf("keywords = %v, want none", hc.Keywords)
				}
			},
		},
		{
			// every field set explicitly; nothing should fall back to a default.
			name: "custom values",
			input: monitor.CreateHTTPConfigInput{
				Scheme: "http", Path: "/custom", Method: "GET",
				Interval: 30, Timeout: 5, MaxAttempts: 5,
				Keywords: []string{"ok", "up"},
			},
			setup:      expectAdd,
			wantEvents: 1,
			check: func(t *testing.T, hc monitor.HTTPConfig) {
				if hc.Interval.Seconds() != 30 {
					t.Errorf("interval = %d, want 30", hc.Interval.Seconds())
				}
				if hc.MaxAttempts.Count() != 5 {
					t.Errorf("maxAttempts = %d, want 5", hc.MaxAttempts.Count())
				}
				if len(hc.Keywords) != 2 {
					t.Errorf("keywords = %v, want 2", hc.Keywords)
				}
			},
		},
		{
			// invalid scheme fails in the domain before any repo write; no event.
			name:       "invalid scheme",
			input:      monitor.CreateHTTPConfigInput{Scheme: "ftp", Path: "/x", Method: "GET"},
			setup:      expectGet,
			wantErr:    monitor.ErrValidation,
			wantEvents: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, events := newSvc(t)
			m, _ := monitor.New(validInput())
			tt.setup(repo, m)

			hc, err := svc.AddHTTPCheck(context.Background(), m.ID, tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("AddHTTPCheck: %v", err)
			}

			if n := len(events); n != tt.wantEvents {
				t.Fatalf("events = %d, want %d", n, tt.wantEvents)
			}

			if tt.check != nil {
				tt.check(t, hc)
			}

			// Every emitted event must be a Created carrying exactly the job
			// projected from the persisted config — this is what reaches the
			// scheduler, so it must match the config we just returned.
			for range tt.wantEvents {
				ev := <-events
				if ev.Type != monitor.EventCreated {
					t.Errorf("type = %v, want EventCreated", ev.Type)
				}
				if !ev.Job.Equal(hc.ToJob(m.Host)) {
					t.Errorf("emitted job %+v does not match created config %+v", ev.Job, hc.ToJob(m.Host))
				}
			}
		})
	}
}

func TestService_Delete(t *testing.T) {
	// expectDelete scripts the happy path: GetMonitor then a successful
	// DeleteMonitor. The error rows swap in a failing call at one of the two
	// points; a GetMonitor failure never reaches DeleteMonitor.
	expectDelete := func(repo *mocks.MockRepository, m monitor.Monitor) {
		repo.EXPECT().GetMonitor(mock.Anything, m.ID).Return(m, nil).Once()
		repo.EXPECT().DeleteMonitor(mock.Anything, m.ID).Return(nil).Once()
	}

	tests := []struct {
		name    string
		setup   func(repo *mocks.MockRepository, m monitor.Monitor)
		wantErr error
		emits   bool // happy path emits one Deleted per enabled config
	}{
		{
			name:  "deletes and emits",
			setup: expectDelete,
			emits: true,
		},
		{
			// commit fails after the read: syncScheduler is never reached, so
			// nothing must be emitted (no event before a successful delete).
			name: "delete fails",
			setup: func(repo *mocks.MockRepository, m monitor.Monitor) {
				repo.EXPECT().GetMonitor(mock.Anything, m.ID).Return(m, nil).Once()
				repo.EXPECT().DeleteMonitor(mock.Anything, m.ID).Return(monitor.ErrMonitorNotFound).Once()
			},
			wantErr: monitor.ErrMonitorNotFound,
		},
		{
			// read fails: DeleteMonitor is never called, nothing emitted.
			name: "get fails",
			setup: func(repo *mocks.MockRepository, m monitor.Monitor) {
				repo.EXPECT().GetMonitor(mock.Anything, m.ID).Return(monitor.Monitor{}, monitor.ErrMonitorNotFound).Once()
			},
			wantErr: monitor.ErrMonitorNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, events := newSvc(t)
			m, _ := monitor.New(validInput())
			tt.setup(repo, m)

			// Ground truth for the happy path: the jobs projected from the
			// monitor's enabled configs, keyed by config id. Delete must emit a
			// Deleted for each. Re-derived here because projectJobs is unexported.
			want := map[uuid.UUID]monitor.CheckJob{}
			if m.PingConfig.IsEnabled {
				want[m.PingConfig.ID] = m.PingConfig.ToJob(m.Host)
			}
			for _, hc := range m.HTTPConfigs {
				if hc.IsEnabled {
					want[hc.ID] = hc.ToJob(m.Host)
				}
			}

			err := svc.Delete(context.Background(), m.ID)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Delete: %v", err)
			}

			wantEvents := 0
			if tt.emits {
				wantEvents = len(want)
			}
			if n := len(events); n != wantEvents {
				t.Fatalf("events = %d, want %d", n, wantEvents)
			}

			// Each event must be a Deleted carrying exactly the job that was
			// live for its config — matched by config id, since map iteration
			// (and thus emission) order is not deterministic.
			for range wantEvents {
				ev := <-events
				if ev.Type != monitor.EventDeleted {
					t.Errorf("type = %v, want EventDeleted", ev.Type)
				}
				cfgID := ev.Job.Base().ConfigID
				expected, ok := want[cfgID]
				if !ok {
					t.Errorf("event for unknown config %v", cfgID)
					continue
				}
				if !ev.Job.Equal(expected) {
					t.Errorf("emitted job %+v does not match live config %+v", ev.Job, expected)
				}
			}
		})
	}
}

func TestService_UpdateHTTPCheck_EnableDisable(t *testing.T) {
	expectUpdate := func(repo *mocks.MockRepository, m monitor.Monitor) {
		repo.EXPECT().GetMonitor(mock.Anything, m.ID).Return(m, nil).Once()
		repo.EXPECT().UpdateHTTPConfig(mock.Anything, mock.AnythingOfType("monitor.HTTPConfig")).
			Return(nil).Once()
	}
	expectGet := func(repo *mocks.MockRepository, m monitor.Monitor) {
		repo.EXPECT().GetMonitor(mock.Anything, m.ID).Return(m, nil).Once()
	}

	tests := []struct {
		name          string
		enabledBefore bool
		input         monitor.UpdateHTTPConfigInput
		targetWrong   bool // aim the update at a config id the monitor does not have
		setup         func(repo *mocks.MockRepository, m monitor.Monitor)
		wantErr       error
		wantType      monitor.EventType
		wantEvents    int
	}{
		{
			// enabled -> disabled: the job drops out of the projection.
			name:          "disable emits Deleted",
			enabledBefore: true,
			input:         monitor.UpdateHTTPConfigInput{IsEnabled: ptr(false)},
			setup:         expectUpdate,
			wantType:      monitor.EventDeleted,
			wantEvents:    1,
		},
		{
			// disabled -> enabled: the job enters the projection.
			name:          "enable emits Created",
			enabledBefore: false,
			input:         monitor.UpdateHTTPConfigInput{IsEnabled: ptr(true)},
			setup:         expectUpdate,
			wantType:      monitor.EventCreated,
			wantEvents:    1,
		},
		{
			// unknown config fails in the domain before any repo write; no event.
			name:          "unknown config",
			enabledBefore: true,
			input:         monitor.UpdateHTTPConfigInput{IsEnabled: ptr(false)},
			targetWrong:   true,
			setup:         expectGet,
			wantErr:       monitor.ErrHTTPConfigNotFound,
			wantEvents:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, events := newSvc(t)
			monID, cfgID := uuid.New(), uuid.New()
			hc := monitor.ReconstructHTTPConfig(cfgID, monID, "https", "/health", "GET", tt.enabledBefore, 60, 10, 3, nil)
			// ping left zero-valued (disabled) so the toggled http config is the
			// only job in play — its event count is unambiguous.
			m := monitor.ReconstructMonitor(monID, "x", "x.com", "", monitor.PingConfig{}, []monitor.HTTPConfig{hc})
			tt.setup(repo, m)

			target := cfgID
			if tt.targetWrong {
				target = uuid.New()
			}

			err := svc.UpdateHTTPCheck(context.Background(), monID, target, tt.input)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("UpdateHTTPCheck: %v", err)
			}

			if n := len(events); n != tt.wantEvents {
				t.Fatalf("events = %d, want %d", n, tt.wantEvents)
			}

			// IsEnabled is not part of the job, so the emitted job matches the
			// fixture config's projection regardless of the toggle direction.
			for range tt.wantEvents {
				ev := <-events
				if ev.Type != tt.wantType {
					t.Errorf("type = %v, want %v", ev.Type, tt.wantType)
				}
				if !ev.Job.Equal(hc.ToJob(m.Host)) {
					t.Errorf("emitted job %+v does not match config %+v", ev.Job, hc.ToJob(m.Host))
				}
			}
		})
	}
}

// Regression for the nil-id UNIQUE collision (#38): every call must mint a
// fresh, non-nil id, so two results built from the *same* input still persist
// as distinct rows instead of colliding on UNIQUE(id).
func TestService_HandleCheckResult_MintsFreshID(t *testing.T) {
	svc, repo, _ := newSvc(t)

	var saved []monitor.CheckResult
	repo.EXPECT().
		SaveCheckResult(mock.Anything, mock.AnythingOfType("monitor.CheckResult")).
		Run(func(_ context.Context, r monitor.CheckResult) { saved = append(saved, r) }).
		Return(nil).
		Times(2)

	in := monitor.CheckResultInput{
		MonitorID: uuid.New(),
		ConfigID:  uuid.New(),
		Reachable: true,
	}
	for i := range 2 {
		if err := svc.HandleCheckResult(context.Background(), in); err != nil {
			t.Fatalf("HandleCheckResult #%d: %v", i, err)
		}
	}

	if saved[0].ID == uuid.Nil || saved[1].ID == uuid.Nil {
		t.Errorf("minted nil id: %v, %v", saved[0].ID, saved[1].ID)
	}
	if saved[0].ID == saved[1].ID {
		t.Errorf("two results share id %v — would collide on UNIQUE(id)", saved[0].ID)
	}
}

// Status is derived in the domain from raw facts. Failure-without-error
// (Reachable=false, Error=nil) is a legitimate case and must not panic.
func TestService_HandleCheckResult_DerivesStatus(t *testing.T) {
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
			svc, repo, _ := newSvc(t)

			var got monitor.CheckResult
			repo.EXPECT().
				SaveCheckResult(mock.Anything, mock.AnythingOfType("monitor.CheckResult")).
				Run(func(_ context.Context, r monitor.CheckResult) { got = r }).
				Return(nil).
				Once()

			err := svc.HandleCheckResult(context.Background(), monitor.CheckResultInput{
				MonitorID: uuid.New(),
				ConfigID:  uuid.New(),
				Reachable: tt.reachable,
				Error:     tt.err,
			})
			if err != nil {
				t.Fatalf("HandleCheckResult: %v", err)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", got.Status, tt.wantStatus)
			}
			if got.Error != tt.wantMsg {
				t.Errorf("Error = %q, want %q", got.Error, tt.wantMsg)
			}
		})
	}
}

func TestService_List_Delegates(t *testing.T) {
	svc, repo, _ := newSvc(t)
	want := []monitor.Monitor{{Name: "a"}, {Name: "b"}}

	repo.EXPECT().GetMonitorList(mock.Anything).Return(want, nil).Once()

	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != len(want) {
		t.Errorf("len = %d, want %d", len(got), len(want))
	}
}
