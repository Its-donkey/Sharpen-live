package ratelimit

import (
	"context"
	"net/http"
	"sync"
	"time"
)

var (
	mu       sync.Mutex
	gate     chan time.Time
	ticker   *time.Ticker
	interval = 5 * time.Second
)

// Wait blocks until the next YouTube request slot is available or the context is canceled.
// The first call is allowed immediately; subsequent calls are spaced by the configured interval.
func Wait(ctx context.Context) error {
	ensureStarted()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-gate:
		return nil
	}
}

// Client wraps the provided HTTP client with a transport that enforces the throttle interval.
// If base is nil, http.DefaultClient is used.
func Client(base *http.Client) *http.Client {
	ensureStarted()
	if base == nil {
		base = &http.Client{}
	}
	rt := base.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	base.Transport = throttledTransport{base: rt}
	return base
}

// SetIntervalForTesting resets the limiter interval. It should only be used in tests.
func SetIntervalForTesting(d time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	if ticker != nil {
		ticker.Stop()
	}
	gate = nil
	ticker = nil
	if d > 0 {
		interval = d
	}
}

type throttledTransport struct {
	base http.RoundTripper
}

func (t throttledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := Wait(req.Context()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func ensureStarted() {
	mu.Lock()
	defer mu.Unlock()
	if gate != nil {
		return
	}
	gate = make(chan time.Time, 1)
	gate <- time.Now()
	ticker = time.NewTicker(interval)
	go func() {
		for t := range ticker.C {
			gate <- t
		}
	}()
}
