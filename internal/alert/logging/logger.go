// Package logging centralises the alert-server logging helpers and adapters.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Logger represents the minimal logging interface used across the project.
type Logger interface {
	Printf(format string, v ...any)
}

// StdLoggerProvider can expose the underlying *log.Logger when needed.
type stdLoggerProvider interface {
	StdLogger() *log.Logger
}

type stdLogger struct {
	base *log.Logger
}

type newlineWriter struct {
	mu      sync.Mutex
	w       io.Writer
	started bool
}

var (
	defaultWriter   io.Writer = os.Stdout
	defaultWriterMu sync.RWMutex
)

// New returns a Logger that writes to stdout without automatic prefixes.
func New() Logger {
	return NewWithWriter(getDefaultWriter())
}

// NewWithWriter builds a Logger that writes to the provided io.Writer and
// inserts a blank line between log entries for readability.
func NewWithWriter(w io.Writer) Logger {
	if w == nil {
		w = os.Stdout
	}
	adapter := &newlineWriter{w: w}
	return &stdLogger{base: log.New(adapter, "", 0)}
}

// SetDefaultWriter overrides the writer used by New().
func SetDefaultWriter(w io.Writer) {
	defaultWriterMu.Lock()
	defer defaultWriterMu.Unlock()
	if w == nil {
		defaultWriter = os.Stdout
		return
	}
	defaultWriter = w
}

// ReplaceLoggerWriter swaps the output of an existing logger to the provided writer.
func ReplaceLoggerWriter(logger Logger, w io.Writer) {
	if logger == nil || w == nil {
		return
	}
	adapter := &newlineWriter{w: w}
	switch l := logger.(type) {
	case *stdLogger:
		if l.base != nil {
			l.base.SetOutput(adapter)
		}
	case stdLoggerProvider:
		if base := l.StdLogger(); base != nil {
			base.SetOutput(adapter)
		}
	}
}

func getDefaultWriter() io.Writer {
	defaultWriterMu.RLock()
	defer defaultWriterMu.RUnlock()
	return defaultWriter
}

// AsStdLogger returns the underlying *log.Logger when available so packages
// like net/http can keep using their native logger type.
func AsStdLogger(logger Logger) *log.Logger {
	if logger == nil {
		return nil
	}
	if provider, ok := logger.(stdLoggerProvider); ok {
		return provider.StdLogger()
	}
	if std, ok := logger.(*stdLogger); ok {
		return std.base
	}
	return nil
}

func (l *stdLogger) Printf(format string, v ...any) {
	if l == nil || l.base == nil {
		return
	}
	msg := fmt.Sprintf(format, v...)
	emitLogEvents(l, logEvent{
		Time:     time.Now().Format("2006/01/02 15:04:05"),
		Message:  msg,
		Category: "general",
	})
}

func (l *stdLogger) StdLogger() *log.Logger {
	if l == nil {
		return nil
	}
	return l.base
}

func (w *newlineWriter) Write(p []byte) (int, error) {
	if w == nil || w.w == nil {
		return len(p), nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		if _, err := w.w.Write([]byte("\n")); err != nil {
			return 0, err
		}
	} else {
		w.started = true
	}
	if len(p) == 0 {
		return 0, nil
	}
	if _, err := w.w.Write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func logJSON(logger Logger, entries ...logEvent) { emitLogEvents(logger, entries...) }

func emitLogEvents(logger Logger, entries ...logEvent) {
	if len(entries) == 0 || logger == nil {
		return
	}
	for i := range entries {
		if entries[i].Time == "" {
			entries[i].Time = time.Now().UTC().Format(time.RFC3339Nano)
		}
		if entries[i].Category == "" {
			entries[i].Category = "general"
		}
		if entries[i].Category == "general" && entries[i].Source == "" {
			if caller := callerLocation(3); caller != "" {
				entries[i].Source = caller
			}
		}
	}
	payload := logPayload{LogEvents: entries}
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal log entry: %v\n", err)
		return
	}

	switch l := logger.(type) {
	case *stdLogger:
		l.base.Print(string(data))
	case stdLoggerProvider:
		if base := l.StdLogger(); base != nil {
			base.Print(string(data))
			break
		}
		logger.Printf("%s", data)
	default:
		logger.Printf("%s", data)
	}

	logStream.Broadcast(data)

	writeCategoryEntries(entries, data)
}

func writeCategoryEntries(entries []logEvent, payload []byte) {
	if len(entries) == 0 {
		return
	}
	category := entries[0].Category
	if category == "" {
		return
	}
	w, ok := categoryWriters.Load(category)
	if !ok {
		return
	}
	writer, ok := w.(WriteCloser)
	if !ok || writer == nil {
		return
	}

	if env, ok := writer.(*envelopeWriter); ok {
		for _, entry := range entries {
			eventData, err := json.Marshal(entry)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to marshal log event for category %s: %v\n", category, err)
				continue
			}
			_, _ = env.Write(eventData)
		}
		return
	}

	_, _ = writer.Write(payload)
}

func callerLocation(skip int) string {
	// skip is passed in from emitLogEvents to land on the caller that invoked
	// the logger (logger.Printf -> emitLogEvents -> runtime.Caller).
	if skip < 0 {
		skip = 0
	}
	if _, file, line, ok := runtime.Caller(skip); ok {
		return fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	return ""
}
