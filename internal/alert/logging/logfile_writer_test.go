package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogFileWriterUsesSingleEnvelope(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.json")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open log file: %v", err)
	}

	writer := NewLogFileWriter(file)
	payload := logPayload{LogEvents: []logEvent{{Message: "first"}, {Message: "second"}}}
	data, _ := json.Marshal(payload)

	if _, err := writer.Write(data); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if strings.Count(string(content), `"logevents":[`) != 1 {
		t.Fatalf("expected single logevents envelope, got %q", content)
	}

	var combined logPayload
	if err := json.Unmarshal(content, &combined); err != nil {
		t.Fatalf("unmarshal combined payload: %v", err)
	}
	if len(combined.LogEvents) != 2 {
		t.Fatalf("expected two events, got %d", len(combined.LogEvents))
	}
}
