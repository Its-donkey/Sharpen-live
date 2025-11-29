// file name — /internal/alert/platforms/youtube/ratelimit/limiter.go
package ratelimit

import (
	"context"
	"net/http"
	"sync"
	"time"
)

var (
	mu       sync.Mutex
	interval = 5 * time.Second
)

// throttledTransport wraps a base RoundTripper and enforces a minimum delay
// between requests using the global interval.
type throttledTransport struct {
	base http.RoundTripper
}

// RoundTrip waits for the configured interval or for the request context to
// cancel before delegating to the underlying RoundTripper.
func (t throttledTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Capture the current interval under lock to avoid races.
	mu.Lock()
	currentInterval := interval
	mu.Unlock()

	if currentInterval > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(currentInterval):
		}
	}

	return t.base.RoundTrip(req)
}

// ensureStarted is kept for API compatibility with the original implementation.
// The new implementation does not require background goroutines, so this is a no-op.
func ensureStarted() {}

// Client wraps the provided HTTP client with a transport that enforces a
// throttle interval. If base is nil, http.DefaultClient is used.
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
	interval = d
}

// Wait blocks for the configured interval or until the context is canceled.
func Wait(ctx context.Context) error {
	mu.Lock()
	delay := interval
	mu.Unlock()
	if delay <= 0 {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}
