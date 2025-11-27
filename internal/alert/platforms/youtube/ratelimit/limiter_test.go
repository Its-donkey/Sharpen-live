package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestWaitRespectsInterval(t *testing.T) {
	SetIntervalForTesting(20 * time.Millisecond)
	defer SetIntervalForTesting(5 * time.Second)

	ctx := context.Background()
	if err := Wait(ctx); err != nil {
		t.Fatalf("first wait failed: %v", err)
	}
	start := time.Now()
	if err := Wait(ctx); err != nil {
		t.Fatalf("second wait failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 18*time.Millisecond {
		t.Fatalf("expected at least ~20ms gap, got %s", elapsed)
	}
}

func TestClientWrapsTransport(t *testing.T) {
	SetIntervalForTesting(0) // reset gate
	defer SetIntervalForTesting(5 * time.Second)
	client := Client(nil)
	if client.Transport == nil {
		t.Fatalf("expected transport to be set")
	}
}
