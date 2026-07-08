package monitor_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/monitor/mocks"
)

// Service-level tests: they verify the Service's own behaviour — that it
// delegates to the Repository and propagates results/errors. The store is a
// generated mock, so we script exactly what the repository returns.

func newSvc(t *testing.T) (*monitor.Service, *mocks.MockRepository) {
	t.Helper()
	repo := mocks.NewMockRepository(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return monitor.NewService(repo, log), repo
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
	svc, repo := newSvc(t)

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
	svc, repo := newSvc(t)

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
	svc, _ := newSvc(t)

	_, err := svc.Create(context.Background(), monitor.CreateMonitorInput{})
	if !errors.Is(err, monitor.ErrMonitorEmptyName) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorEmptyName)
	}
}

func TestService_Get_Propagates(t *testing.T) {
	svc, repo := newSvc(t)
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
	svc, repo := newSvc(t)
	id := uuid.New()
	name := "new"

	repo.EXPECT().
		UpdateMonitor(mock.Anything, id, mock.AnythingOfType("monitor.UpdateMonitorInput")).
		Return(monitor.ErrMonitorNotFound).
		Once()

	err := svc.Update(context.Background(), id, monitor.UpdateMonitorInput{Name: &name})
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

func TestService_Delete_Propagates(t *testing.T) {
	svc, repo := newSvc(t)
	id := uuid.New()

	repo.EXPECT().DeleteMonitor(mock.Anything, id).
		Return(monitor.ErrMonitorNotFound).
		Once()

	err := svc.Delete(context.Background(), id)
	if !errors.Is(err, monitor.ErrMonitorNotFound) {
		t.Errorf("err = %v, want %v", err, monitor.ErrMonitorNotFound)
	}
}

// Regression for the nil-id UNIQUE collision (#38): every call must mint a
// fresh, non-nil id, so two results built from the *same* input still persist
// as distinct rows instead of colliding on UNIQUE(id).
func TestService_HandleCheckResult_MintsFreshID(t *testing.T) {
	svc, repo := newSvc(t)

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
	for i := 0; i < 2; i++ {
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
			svc, repo := newSvc(t)

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
	svc, repo := newSvc(t)
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
