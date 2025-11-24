// Package logging centralises the alert-server logging helpers and adapters.
package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
	logStream       = newLogStream()
)

const maxLoggedResponseBody = 4096

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
		Time:    time.Now().Format("2006/01/02 15:04:05"),
		Message: msg,
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

// WithHTTPLogging wraps the provided handler so every request/response pair is logged.
func WithHTTPLogging(next http.Handler, logger Logger) http.Handler {
	if logger == nil || next == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.ToUpper(uuid.New().String())
		events := make([]logEvent, 0, 2)
		now := time.Now().UTC()
		if dump, err := httputil.DumpRequest(r, true); err == nil {
			events = append(events, logEvent{
				Time:      now.Format(time.RFC3339Nano),
				ID:        requestID,
				Direction: "request",
				Message:   fmt.Sprintf("Incoming request from %s", r.RemoteAddr),
				Raw:       string(dump),
				Method:    r.Method,
				Path:      r.URL.Path,
				Remote:    r.RemoteAddr,
			})
		} else {
			logger.Printf("failed to dump request from %s: %v", r.RemoteAddr, err)
		}

		lrw := newLoggingResponseWriter(w)
		defer func() {
			status := lrw.StatusCode()
			responseEvent := logEvent{
				Time:      time.Now().UTC().Format(time.RFC3339Nano),
				ID:        requestID,
				Direction: "response",
				Message:   fmt.Sprintf("Response for %s %s (%d %s)", r.Method, r.URL.Path, status, http.StatusText(status)),
				Raw:       lrw.LoggedBody(),
				Method:    r.Method,
				Path:      r.URL.Path,
				Status:    status,
				Remote:    r.RemoteAddr,
			}
			events = append(events, responseEvent)
			logJSON(logger, events...)
		}()

		next.ServeHTTP(lrw, r)
	})
}

type logEvent struct {
	Time      string `json:"time"`
	ID        string `json:"id,omitempty"`
	Direction string `json:"direction,omitempty"`
	Message   string `json:"message"`
	Raw       string `json:"raw,omitempty"`
	Method    string `json:"method,omitempty"`
	Path      string `json:"path,omitempty"`
	Status    int    `json:"status,omitempty"`
	Remote    string `json:"remote,omitempty"`
}

type logPayload struct {
	LogEvents []logEvent `json:"logevents"`
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
}

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

// Subscribe returns a channel of log entries and a snapshot of current logs.
func Subscribe() (chan []byte, [][]byte) { return logStream.Subscribe() }

// Unsubscribe removes a previously subscribed channel.
func Unsubscribe(ch chan []byte) { logStream.Unsubscribe(ch) }

type logStreamState struct {
	mu          sync.RWMutex
	buffer      [][]byte
	subscribers map[chan []byte]struct{}
}

func newLogStream() *logStreamState {
	return &logStreamState{
		buffer:      make([][]byte, 0, maxStoredLogEntries),
		subscribers: make(map[chan []byte]struct{}),
	}
}

const maxStoredLogEntries = 300

func (l *logStreamState) Broadcast(entry []byte) {
	if l == nil || entry == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buffer) >= maxStoredLogEntries {
		copy(l.buffer, l.buffer[1:])
		l.buffer[len(l.buffer)-1] = append([]byte(nil), entry...)
	} else {
		l.buffer = append(l.buffer, append([]byte(nil), entry...))
	}
	for ch := range l.subscribers {
		select {
		case ch <- entry:
		default:
		}
	}
}

func (l *logStreamState) Subscribe() (chan []byte, [][]byte) {
	ch := make(chan []byte, 64)
	l.mu.RLock()
	snapshot := make([][]byte, len(l.buffer))
	for i := range l.buffer {
		snapshot[i] = append([]byte(nil), l.buffer[i]...)
	}
	l.mu.RUnlock()

	l.mu.Lock()
	l.subscribers[ch] = struct{}{}
	l.mu.Unlock()
	return ch, snapshot
}

func (l *logStreamState) Unsubscribe(ch chan []byte) {
	if l == nil || ch == nil {
		return
	}
	l.mu.Lock()
	if _, ok := l.subscribers[ch]; ok {
		delete(l.subscribers, ch)
		close(ch)
	}
	l.mu.Unlock()
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status    int
	buf       bytes.Buffer
	truncated bool
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{ResponseWriter: w}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.status == 0 {
		lrw.status = http.StatusOK
	}
	if lrw.buf.Len() < maxLoggedResponseBody {
		remaining := maxLoggedResponseBody - lrw.buf.Len()
		if len(b) > remaining {
			lrw.buf.Write(b[:remaining])
			lrw.truncated = true
		} else {
			lrw.buf.Write(b)
		}
	} else {
		lrw.truncated = true
	}
	return lrw.ResponseWriter.Write(b)
}

func (lrw *loggingResponseWriter) StatusCode() int {
	if lrw.status == 0 {
		return http.StatusOK
	}
	return lrw.status
}

func (lrw *loggingResponseWriter) LoggedBody() string {
	body := lrw.buf.String()
	if lrw.truncated {
		return fmt.Sprintf("%s\n-- response truncated after %d bytes --", body, maxLoggedResponseBody)
	}
	return body
}

func (lrw *loggingResponseWriter) Flush() {
	if flusher, ok := lrw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
