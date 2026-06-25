package scheduler

import (
	"container/heap"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/services/checker"
)

// goleak fails the suite if any goroutine outlives the tests — the whole point
// of a worker-pool scheduler is that Start/Stop is leak-free.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// --- fakes ---------------------------------------------------------------

type fakeGetter struct {
	checks []monitor.RunnableCheck
	err    error
}

func (f *fakeGetter) ListEnabledCheckConfigs(context.Context) ([]monitor.RunnableCheck, error) {
	return f.checks, f.err
}

type fakeChecker struct {
	result checker.CheckResult
	err    error

	mu    sync.Mutex
	calls int
}

func (c *fakeChecker) Check(_ context.Context, _ checker.CheckRequest) (checker.CheckResult, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return c.result, c.err
}

func (c *fakeChecker) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// blockingChecker blocks inside Check until released, then reports whether its
// context was cancelled. It proves an in-flight check is allowed to finish on
// Stop instead of being cancelled into a failure.
type blockingChecker struct {
	entered chan struct{}
	once    sync.Once
	release chan struct{}
}

func (c *blockingChecker) Check(ctx context.Context, _ checker.CheckRequest) (checker.CheckResult, error) {
	c.once.Do(func() { close(c.entered) })
	<-c.release
	if err := ctx.Err(); err != nil {
		return checker.CheckResult{}, err // cancelled mid-flight -> would become a failure
	}
	return checker.CheckResult{Reachable: true, StatusCode: 200}, nil
}

// fakeHandler forwards every handled result onto a buffered channel so tests can
// observe completions without depending on wall-clock timing.
type fakeHandler struct {
	results chan monitor.MonitorCheckResult
}

func newFakeHandler() *fakeHandler {
	return &fakeHandler{results: make(chan monitor.MonitorCheckResult, 1024)}
}

func (h *fakeHandler) HandleCheckResult(ctx context.Context, r monitor.MonitorCheckResult) error {
	select {
	case h.results <- r:
	case <-ctx.Done():
	}
	return nil
}

// --- helpers -------------------------------------------------------------

func runnable(ct monitor.CheckType, interval time.Duration) monitor.RunnableCheck {
	return monitor.RunnableCheck{
		MonitorID: uuid.New(),
		ConfigID:  uuid.New(),
		URL:       "http://example.test",
		CheckType: ct,
		Interval:  interval,
		Timeout:   time.Second,
	}
}

func newTestScheduler(g ConfigsGetter, h ResultHandler, checkers map[monitor.CheckType]checker.Checker) *Scheduler {
	return New(Params{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Getter:       g,
		Handler:      h,
		Workers:      4,
		Checkers:     checkers,
		WriteTimeout: time.Second,
	})
}

// startAndCleanup starts the scheduler and registers a bounded Stop so a hung
// shutdown fails the test instead of blocking the whole run.
func startAndCleanup(t *testing.T, s *Scheduler) {
	t.Helper()
	require.NoError(t, s.Start(context.Background()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.NoError(t, s.Stop(ctx))
	})
}

// recvResult waits for one handled result or fails after timeout.
func recvResult(t *testing.T, h *fakeHandler, timeout time.Duration) monitor.MonitorCheckResult {
	t.Helper()
	select {
	case r := <-h.results:
		return r
	case <-time.After(timeout):
		t.Fatal("timed out waiting for a check result")
		return monitor.MonitorCheckResult{}
	}
}

// --- pure heap tests (deterministic, no timing) --------------------------

func TestStateHeap_PopsInDueOrder(t *testing.T) {
	base := time.Now()
	offsets := []time.Duration{300, 100, 200, 50, 150}

	h := StateHeap{}
	heap.Init(&h)
	for _, off := range offsets {
		heap.Push(&h, &State{nextDue: base.Add(off * time.Millisecond)})
	}

	var got []time.Duration
	for h.Len() > 0 {
		s := heap.Pop(&h).(*State)
		got = append(got, s.nextDue.Sub(base))
	}

	want := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		150 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
	}
	assert.Equal(t, want, got)
}

func TestStateHeap_PopResetsIndex(t *testing.T) {
	h := StateHeap{}
	heap.Init(&h)
	st := &State{nextDue: time.Now()}
	heap.Push(&h, st)
	assert.Equal(t, 0, st.index)

	popped := heap.Pop(&h).(*State)
	assert.Same(t, st, popped)
	assert.Equal(t, -1, popped.index)
	assert.Equal(t, 0, h.Len())
}

// --- scheduler integration tests -----------------------------------------

func TestScheduler_DispatchesAndRecordsSuccess(t *testing.T) {
	chk := &fakeChecker{result: checker.CheckResult{
		Reachable:    true,
		StatusCode:   200,
		ResponseTime: 5 * time.Millisecond,
	}}
	rc := runnable(monitor.CheckHTTP, 50*time.Millisecond)
	h := newFakeHandler()

	s := newTestScheduler(
		&fakeGetter{checks: []monitor.RunnableCheck{rc}},
		h,
		map[monitor.CheckType]checker.Checker{monitor.CheckHTTP: chk},
	)
	startAndCleanup(t, s)

	r := recvResult(t, h, 2*time.Second)
	assert.Equal(t, rc.ConfigID, r.ConfigID)
	assert.Equal(t, rc.MonitorID, r.MonitorID)
	assert.Equal(t, monitor.CheckSuccess, r.Status)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, 5*time.Millisecond, r.ResponseTime)
}

func TestScheduler_ReschedulesPeriodically(t *testing.T) {
	chk := &fakeChecker{result: checker.CheckResult{Reachable: true}}
	rc := runnable(monitor.CheckHTTP, 30*time.Millisecond)
	h := newFakeHandler()

	s := newTestScheduler(
		&fakeGetter{checks: []monitor.RunnableCheck{rc}},
		h,
		map[monitor.CheckType]checker.Checker{monitor.CheckHTTP: chk},
	)
	startAndCleanup(t, s)

	// A single config on a 30ms interval should fire several times within 1s.
	const want = 3
	deadline := time.After(1 * time.Second)
	got := 0
	for got < want {
		select {
		case <-h.results:
			got++
		case <-deadline:
			t.Fatalf("expected at least %d periodic runs, got %d", want, got)
		}
	}
	assert.GreaterOrEqual(t, got, want)
}

func TestScheduler_RunsAllEnabledConfigs(t *testing.T) {
	chk := &fakeChecker{result: checker.CheckResult{Reachable: true}}
	rc1 := runnable(monitor.CheckHTTP, 40*time.Millisecond)
	rc2 := runnable(monitor.CheckPing, 40*time.Millisecond)
	h := newFakeHandler()

	s := newTestScheduler(
		&fakeGetter{checks: []monitor.RunnableCheck{rc1, rc2}},
		h,
		map[monitor.CheckType]checker.Checker{
			monitor.CheckHTTP: chk,
			monitor.CheckPing: chk,
		},
	)
	startAndCleanup(t, s)

	seen := map[uuid.UUID]bool{}
	deadline := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case r := <-h.results:
			seen[r.ConfigID] = true
		case <-deadline:
			t.Fatalf("expected both configs to run, saw %d", len(seen))
		}
	}
	assert.True(t, seen[rc1.ConfigID])
	assert.True(t, seen[rc2.ConfigID])
}

func TestScheduler_UnknownCheckerType_RecordsFailure(t *testing.T) {
	rc := runnable(monitor.CheckHeadless, 50*time.Millisecond)
	h := newFakeHandler()

	// No checker registered for CheckHeadless.
	s := newTestScheduler(
		&fakeGetter{checks: []monitor.RunnableCheck{rc}},
		h,
		map[monitor.CheckType]checker.Checker{},
	)
	startAndCleanup(t, s)

	r := recvResult(t, h, 2*time.Second)
	assert.Equal(t, monitor.CheckFailure, r.Status)
	assert.Contains(t, r.Error, "checker")
}

func TestScheduler_CheckerError_RecordsFailure(t *testing.T) {
	chk := &fakeChecker{err: errors.New("boom")}
	rc := runnable(monitor.CheckHTTP, 50*time.Millisecond)
	h := newFakeHandler()

	s := newTestScheduler(
		&fakeGetter{checks: []monitor.RunnableCheck{rc}},
		h,
		map[monitor.CheckType]checker.Checker{monitor.CheckHTTP: chk},
	)
	startAndCleanup(t, s)

	r := recvResult(t, h, 2*time.Second)
	assert.Equal(t, monitor.CheckFailure, r.Status)
	assert.Contains(t, r.Error, "boom")
}

func TestScheduler_NoConfigs_StartsAndStopsCleanly(t *testing.T) {
	s := newTestScheduler(&fakeGetter{}, newFakeHandler(), map[monitor.CheckType]checker.Checker{})
	startAndCleanup(t, s) // start succeeds, Cleanup asserts a clean Stop; goleak asserts no leak
}

func TestScheduler_GetterError_StartFails(t *testing.T) {
	s := newTestScheduler(
		&fakeGetter{err: errors.New("db down")},
		newFakeHandler(),
		map[monitor.CheckType]checker.Checker{},
	)
	err := s.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestScheduler_StopReturnsBeforeDeadline(t *testing.T) {
	chk := &fakeChecker{result: checker.CheckResult{Reachable: true}}
	rc := runnable(monitor.CheckHTTP, 30*time.Millisecond)
	s := newTestScheduler(
		&fakeGetter{checks: []monitor.RunnableCheck{rc}},
		newFakeHandler(),
		map[monitor.CheckType]checker.Checker{monitor.CheckHTTP: chk},
	)
	require.NoError(t, s.Start(context.Background()))

	// let it run a little so there is in-flight scheduling state to tear down
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	require.NoError(t, s.Stop(ctx))
	assert.Less(t, time.Since(start), 2*time.Second)
}

// Stop before Start is a no-op: the cancel funcs are still nil, so Stop must
// return without panicking.
func TestScheduler_StopBeforeStart(t *testing.T) {
	s := newTestScheduler(&fakeGetter{}, newFakeHandler(), map[monitor.CheckType]checker.Checker{})
	assert.NoError(t, s.Stop(context.Background()))
}

// An in-flight check must be allowed to finish on Stop, not cancelled into a
// spurious failure. Stop cancels the dispatcher immediately but leaves running
// checks on a separate context until the shutdown budget is exhausted.
func TestScheduler_InFlightCheckDrainsOnStop(t *testing.T) {
	chk := &blockingChecker{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	h := newFakeHandler()
	s := newTestScheduler(
		&fakeGetter{checks: []monitor.RunnableCheck{runnable(monitor.CheckHTTP, 10*time.Millisecond)}},
		h,
		map[monitor.CheckType]checker.Checker{monitor.CheckHTTP: chk},
	)
	require.NoError(t, s.Start(context.Background()))

	// wait until a check is actually running inside a worker
	select {
	case <-chk.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("check never entered")
	}

	stopErr := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopErr <- s.Stop(ctx)
	}()

	// give Stop time to cancel the dispatcher before the check completes, so the
	// assertion exercises the "cancel arrived mid-flight" path
	time.Sleep(100 * time.Millisecond)
	close(chk.release)

	require.NoError(t, <-stopErr)

	r := recvResult(t, h, 2*time.Second)
	assert.Equal(t, monitor.CheckSuccess, r.Status,
		"in-flight check was cancelled into a failure instead of draining")
}
