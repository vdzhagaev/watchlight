package scheduler

import (
	"container/heap"
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vdzhagaev/watchlight/internal/lib/logger/sl"
	"github.com/vdzhagaev/watchlight/internal/monitor"
	"github.com/vdzhagaev/watchlight/internal/services/checker"
)

const defaultWorkers = 10

type State struct {
	checkJob monitor.CheckJob
	nextDue  time.Time
	inFlight bool
	index    int
}

type Event struct {
	ConfigID    uuid.UUID
	CompletedAt time.Time
}

type Scheduler struct {
	wg             sync.WaitGroup
	jobs           chan monitor.CheckJob
	results        chan monitor.CheckResultInput
	events         chan Event
	done           chan struct{}
	dispatchCancel context.CancelFunc
	checkCancel    context.CancelFunc
	states         map[uuid.UUID]*State // uuid = configID
	heap           StateHeap
	logger         *slog.Logger
	getter         ConfigsGetter
	handler        ResultHandler
	workers        int
	checkers       map[monitor.CheckType]checker.Checker
	writeTimeout   time.Duration
}

type Params struct {
	Logger       *slog.Logger
	Getter       ConfigsGetter
	Handler      ResultHandler
	Workers      int
	Checkers     map[monitor.CheckType]checker.Checker
	WriteTimeout time.Duration
}

type ConfigsGetter interface {
	ListEnabledCheckConfigs(context.Context) ([]monitor.CheckJob, error)
}

type ResultHandler interface {
	HandleCheckResult(context.Context, monitor.CheckResultInput) error
}

func New(p Params) *Scheduler {
	if p.Workers == 0 {
		p.Workers = defaultWorkers
	}
	if p.WriteTimeout == 0 {
		p.WriteTimeout = 5 * time.Second
	}
	return &Scheduler{
		jobs:         make(chan monitor.CheckJob),
		results:      make(chan monitor.CheckResultInput, p.Workers),
		events:       make(chan Event),
		done:         make(chan struct{}),
		getter:       p.Getter,
		handler:      p.Handler,
		workers:      p.Workers,
		logger:       p.Logger,
		checkers:     p.Checkers,
		writeTimeout: p.WriteTimeout,
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	dispatchCtx, dispatchCancel := context.WithCancel(ctx)
	checkCtx, checkCancel := context.WithCancel(ctx)
	s.dispatchCancel = dispatchCancel
	s.checkCancel = checkCancel

	entries, err := s.buildEntries(ctx)
	if err != nil {
		return fmt.Errorf("scheduler initial load: %w", err)
	}
	s.states = entries
	s.heap = make(StateHeap, 0, len(entries))
	for _, v := range entries {
		v.index = len(s.heap)
		s.heap = append(s.heap, v)
	}
	heap.Init(&s.heap)

	go s.schedule(dispatchCtx)

	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker(checkCtx)
	}

	go s.consume(dispatchCtx)

	go func() {
		s.wg.Wait()
		close(s.results)
	}()

	return nil
}

func (s *Scheduler) Stop(ctx context.Context) error {
	if s.dispatchCancel == nil {
		return nil
	}

	s.dispatchCancel()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		s.checkCancel()
		<-s.done
		return ctx.Err()
	}
}

func (s *Scheduler) schedule(ctx context.Context) {
	events := s.events
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	defer close(s.jobs)

	for {
		s.dispatch(ctx)
		timer.Reset(s.sleepUntilNext())
		select {
		case <-timer.C:
		case ev, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			s.applyEvent(ev)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) dispatch(ctx context.Context) {
	now := time.Now()
	for s.heap.Len() > 0 && !s.heap[0].nextDue.After(now) {
		state := heap.Pop(&s.heap).(*State)
		state.inFlight = true
		select {
		case s.jobs <- state.checkJob:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) applyEvent(ev Event) {
	state, ok := s.states[ev.ConfigID]
	if !ok {
		return
	}
	state.inFlight = false
	state.nextDue = ev.CompletedAt.Add(state.checkJob.Base().Interval)
	heap.Push(&s.heap, state)
}

func (s *Scheduler) buildEntries(ctx context.Context) (map[uuid.UUID]*State, error) {
	cfgs, err := s.getter.ListEnabledCheckConfigs(ctx)
	if err != nil {
		return nil, err
	}
	configs := make(map[uuid.UUID]*State)

	for i, c := range cfgs {
		jitterWindow := int64(min(c.Base().Interval, 30*time.Second))
		nextDue := time.Now().Add(time.Duration(rand.Int63n(jitterWindow)))
		configs[c.Base().ConfigID] = &State{
			checkJob: c,
			nextDue:  nextDue,
			inFlight: false,
			index:    i,
		}
	}

	return configs, nil
}

func (s *Scheduler) sleepUntilNext() time.Duration {
	if s.heap.Len() == 0 {
		return time.Hour
	}

	d := time.Until(s.heap[0].nextDue)
	if d < 0 {
		return 0
	}
	return d
}

func (s *Scheduler) worker(ctx context.Context) {
	defer s.wg.Done()
	for rc := range s.jobs {
		c, ok := s.checkers[rc.CheckType()]

		result := monitor.CheckResultInput{
			MonitorID: rc.Base().MonitorID,
			ConfigID:  rc.Base().ConfigID,
		}

		if !ok {
			result.CheckedAt = time.Now()
			// TODO: Do SchedulerErrors and chage error type from string to `error`
			result.Error = fmt.Errorf("Not available checker for this config: %s", rc.CheckType())

			s.results <- result
			continue
		}
		keywords := make([]string, 0)
		if hj, ok := rc.(monitor.HTTPJob); ok {
			keywords = hj.Keywords
		}
		res, err := c.Check(ctx, checker.CheckRequest{
			Target:   rc.Target(),
			Timeout:  rc.Base().Timeout,
			Keywords: keywords,
		})
		result.CheckedAt = time.Now()
		if err != nil {
			result.Error = err
			s.results <- result
			continue
		}

		result.Reachable = res.Reachable
		result.StatusCode = res.StatusCode
		result.ResponseTime = res.ResponseTime
		result.Error = res.Err
		result.FoundKeywords = res.FoundKeywords
		// result.ScreenshotPath TODO: do later

		s.results <- result
	}
}

func (s *Scheduler) consume(ctx context.Context) {
	defer close(s.events)
	defer close(s.done)
	for r := range s.results {
		writeCtx, cancel := context.WithTimeout(context.Background(), s.writeTimeout)
		err := s.handler.HandleCheckResult(writeCtx, r)
		if err != nil {
			s.logger.Error("write result failed. do something with it later!.", sl.Err(err))
		}
		cancel()
		ev := Event{
			ConfigID:    r.ConfigID,
			CompletedAt: r.CheckedAt,
		}

		select {
		case s.events <- ev:
		case <-ctx.Done():
		}
	}
}
