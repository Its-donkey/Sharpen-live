package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// NewLogFileWriter wraps the provided file so all log events stay inside one
// {"logevents":[...]} envelope and the file remains valid JSON while the server runs.
func NewLogFileWriter(file *os.File) io.WriteCloser {
	writer := &logFileWriter{file: file}
	if file != nil {
		if info, err := file.Stat(); err == nil && info.Size() > 0 {
			writer.started = true
			emptyEnvelopeSize := int64(len(`{"logevents":[]}`) + 1) // include trailing newline
			if info.Size() > emptyEnvelopeSize {
				writer.wroteEntry = true
			}
		}
	}
	return writer
}

const (
	logEnvelopePrefix = `{"logevents":[` + "\n"
	logEnvelopeSuffix = "\n]}\n"
)

type logFileWriter struct {
	mu         sync.Mutex
	file       *os.File
	started    bool
	wroteEntry bool
}

func (w *logFileWriter) Write(p []byte) (int, error) {
	if w == nil || w.file == nil {
		return len(p), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	trimmed := bytes.TrimSpace(p)
	if len(trimmed) == 0 {
		return len(p), nil
	}

	var payload logPayload
	if err := json.Unmarshal(trimmed, &payload); err == nil && len(payload.LogEvents) > 0 {
		if err := w.writeEntries(payload.LogEvents); err != nil {
			return len(p), err
		}
		return len(p), nil
	}

	// Fallback: treat the raw line as a message if it isn't a well-formed payload.
	fallback := logEvent{
		Time:    time.Now().UTC().Format(time.RFC3339Nano),
		Message: string(trimmed),
	}
	if err := w.writeEntries([]logEvent{fallback}); err != nil {
		return len(p), err
	}
	return len(p), nil
}

func (w *logFileWriter) writeEntries(entries []logEvent) error {
	if len(entries) == 0 {
		return nil
	}
	if !w.started {
		// Start the envelope with an empty array so the file is valid JSON immediately.
		if _, err := w.file.Write([]byte(logEnvelopePrefix + strings.TrimPrefix(logEnvelopeSuffix, "\n"))); err != nil {
			return err
		}
		w.started = true
	}
	for i, entry := range entries {
		if _, err := w.file.Seek(-int64(len(logEnvelopeSuffix)), io.SeekEnd); err != nil {
			return err
		}
		if w.wroteEntry || i > 0 {
			if _, err := w.file.Write([]byte(",\n")); err != nil {
				return err
			}
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := w.file.Write(data); err != nil {
			return err
		}
		if _, err := w.file.Write([]byte(logEnvelopeSuffix)); err != nil {
			return err
		}
		w.wroteEntry = true
	}
	return nil
}

func (w *logFileWriter) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return w.file.Close()
	}
	// If no entries were written, emit an empty envelope so the file is still valid JSON.
	if _, err := w.file.Write([]byte(`{"logevents":[]}` + "\n")); err != nil {
		_ = w.file.Close()
		return err
	}
	return w.file.Close()
}
