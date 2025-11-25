package logging

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCategoryWriterStoresIndividualEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "http.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open log file: %v", err)
	}
	writer := NewCategoryLogFileWriter(file)
	SetCategoryWriter("http", writer)
	t.Cleanup(func() { SetCategoryWriter("http", nil) })

	logger := NewWithWriter(io.Discard)
	emitLogEvents(logger,
		logEvent{Category: "http", Message: "first"},
		logEvent{Category: "http", Message: "second"},
	)
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read http log: %v", err)
	}
	if strings.Count(string(content), `"logevents":[`) != 1 {
		t.Fatalf("expected single logevents envelope, got %q", content)
	}

	var payload logPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("unmarshal log payload: %v", err)
	}
	if len(payload.LogEvents) != 2 {
		t.Fatalf("expected two log events, got %d", len(payload.LogEvents))
	}
	if payload.LogEvents[0].Message != "first" || payload.LogEvents[1].Message != "second" {
		t.Fatalf("unexpected messages: %+v", payload.LogEvents)
	}
}
