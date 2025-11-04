package alerts

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeTicker struct {
	ch   chan time.Time
	stop chan struct{}
}

func newFakeTicker() *fakeTicker {
	return &fakeTicker{
		ch:   make(chan time.Time),
		stop: make(chan struct{}),
	}
}

func (t *fakeTicker) C() <-chan time.Time {
	return t.ch
}

func (t *fakeTicker) Stop() {
	close(t.stop)
	close(t.ch)
}

func (t *fakeTicker) tick() {
	select {
	case <-t.stop:
	case t.ch <- time.Now():
	}
}

type fakeChecker struct {
	mu        sync.Mutex
	responses []bool
	err       error
	calls     int
}

func (f *fakeChecker) IsLive(_ context.Context, _, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return false, f.err
	}

	if f.calls >= len(f.responses) {
		return false, nil
	}

	result := f.responses[f.calls]
	f.calls++
	return result, nil
}

func TestHandleMissingChannel(t *testing.T) {
	monitor := NewMonitor(MonitorConfig{Checker: &fakeChecker{}, Interval: time.Minute})
	err := monitor.Handle(context.Background(), StreamAlert{Status: "online"})
	if !errors.Is(err, ErrMissingChannelID) {
		t.Fatalf("expected ErrMissingChannelID, got %v", err)
	}
}

func TestHandleStartsAndStopsWatcher(t *testing.T) {
	ticker := newFakeTicker()
	checker := &fakeChecker{responses: []bool{true, false}}
	monitor := NewMonitor(MonitorConfig{
		Checker:       checker,
		Interval:      time.Hour,
		TickerFactory: func(time.Duration) Ticker { return ticker },
	})

	if err := monitor.Handle(context.Background(), StreamAlert{ChannelID: "abc", Status: "online"}); err != nil {
		t.Fatalf("handle returned error: %v", err)
	}

	waitForCondition(t, time.Second, func() bool { return monitor.ActiveWatchers() == 1 })

	ticker.tick()
	waitForCondition(t, time.Second, func() bool { return monitor.ActiveWatchers() == 0 })
}

func TestHandleCancelsWatcherOnStatusChange(t *testing.T) {
	checker := &fakeChecker{responses: []bool{true}}
	monitor := NewMonitor(MonitorConfig{Checker: checker, Interval: time.Minute})

	if err := monitor.Handle(context.Background(), StreamAlert{ChannelID: "xyz", Status: "online"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForCondition(t, time.Second, func() bool { return monitor.ActiveWatchers() == 1 })

	if err := monitor.Handle(context.Background(), StreamAlert{ChannelID: "xyz", Status: "offline"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	waitForCondition(t, time.Second, func() bool { return monitor.ActiveWatchers() == 0 })
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}
