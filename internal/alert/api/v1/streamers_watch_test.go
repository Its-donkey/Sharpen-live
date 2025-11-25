package v1

import (
	"bufio"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStreamersWatchEmitsDefaultMessageEvents(t *testing.T) {
	dir := t.TempDir()
	streamersFile := filepath.Join(dir, "streamers.json")
	if err := os.WriteFile(streamersFile, []byte(`{"streamers":[]}`), 0o644); err != nil {
		t.Fatalf("write streamers file: %v", err)
	}

	srv := httptest.NewServer(streamersWatchHandler(streamersWatchOptions{
		FilePath:     streamersFile,
		PollInterval: 5 * time.Millisecond,
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Timeout = 2 * time.Second

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	nextEvent := func(t *testing.T) string {
		t.Helper()
		type result struct {
			event string
			err   error
		}
		ch := make(chan result, 1)
		go func() {
			evt, err := readSSEEvent(reader)
			ch <- result{evt, err}
		}()
		select {
		case res := <-ch:
			if res.err != nil {
				t.Fatalf("read sse event: %v", res.err)
			}
			return res.event
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for sse event")
			return ""
		}
	}

	first := nextEvent(t)
	if !strings.HasPrefix(first, "data: ") {
		t.Fatalf("unexpected first event payload: %q", first)
	}
	if strings.Contains(first, "event:") {
		t.Fatalf("first event should use default type, got %q", first)
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(streamersFile, []byte(`{"streamers":["update"]}`), 0o644); err != nil {
		t.Fatalf("update streamers file: %v", err)
	}

	second := nextEvent(t)
	if !strings.HasPrefix(second, "data: ") {
		t.Fatalf("unexpected second event payload: %q", second)
	}
	if strings.Contains(second, "event:") {
		t.Fatalf("second event should use default type, got %q", second)
	}
	if first == second {
		t.Fatalf("expected change event payload to differ after file update")
	}
}

func readSSEEvent(r *bufio.Reader) (string, error) {
	var lines []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}
