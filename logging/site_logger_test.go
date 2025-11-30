package logging

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSiteLoggerIsPerInstance(t *testing.T) {
	dir := t.TempDir()
	oneDir := filepath.Join(dir, "one")
	twoDir := filepath.Join(dir, "two")

	one, err := NewSiteLogger(oneDir, "one")
	if err != nil {
		t.Fatalf("logger one: %v", err)
	}
	two, err := NewSiteLogger(twoDir, "two")
	if err != nil {
		t.Fatalf("logger two: %v", err)
	}

	one.Generalf("hello from %s", "one")
	two.Generalf("hello from %s", "two")

	oneMessages := readGeneralMessages(t, filepath.Join(oneDir, "general.json"))
	twoMessages := readGeneralMessages(t, filepath.Join(twoDir, "general.json"))

	if len(oneMessages) != 1 || oneMessages[0] != "hello from one" {
		t.Fatalf("unexpected messages for logger one: %+v", oneMessages)
	}
	if len(twoMessages) != 1 || twoMessages[0] != "hello from two" {
		t.Fatalf("unexpected messages for logger two: %+v", twoMessages)
	}
}

func TestHTTPLoggingRecordsRequests(t *testing.T) {
	dir := t.TempDir()
	logger, err := NewSiteLogger(dir, "site-a")
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("ok"))
	})

	handler := WithHTTPLogging(logger, mux)

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "127.0.0.1:9999"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	httpEvents := readHTTPEvents(t, filepath.Join(dir, "http.json"))
	if len(httpEvents) != 1 {
		t.Fatalf("expected 1 http event, got %d", len(httpEvents))
	}
	ev := httpEvents[0]
	if ev.Status != http.StatusTeapot || ev.Path != "/ping" || ev.Method != http.MethodGet || ev.Remote != "127.0.0.1" || ev.Site != "site-a" {
		t.Fatalf("unexpected http event: %+v", ev)
	}
}

func readGeneralMessages(t *testing.T, path string) []string {
	t.Helper()
	payload := struct {
		LogEvents []struct {
			Message string `json:"message"`
		} `json:"logevents"`
	}{}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read general log: %v", err)
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal general log: %v", err)
	}
	out := make([]string, 0, len(payload.LogEvents))
	for _, entry := range payload.LogEvents {
		out = append(out, entry.Message)
	}
	return out
}

func readHTTPEvents(t *testing.T, path string) []httpLogEvent {
	t.Helper()
	payload := struct {
		LogEvents []httpLogEvent `json:"logevents"`
	}{}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read http log: %v", err)
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal http log: %v", err)
	}
	return payload.LogEvents
}
