package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewWithWriterOutputsJSONEnvelope(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf)
	logger.Printf("hello %s", "world")
	raw := bytes.TrimSpace(buf.Bytes())
	if bytes.HasPrefix(raw, []byte("\n")) {
		t.Fatalf("expected first byte to be part of JSON, got leading newline: %q", raw)
	}
	var payload logPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal log payload: %v", err)
	}
	if len(payload.LogEvents) != 1 {
		t.Fatalf("expected one log event, got %d", len(payload.LogEvents))
	}
	if payload.LogEvents[0].Message != "hello world" {
		t.Fatalf("expected message to be formatted, got %q", payload.LogEvents[0].Message)
	}
}

func TestSetDefaultWriterAffectsNew(t *testing.T) {
	var buf bytes.Buffer
	SetDefaultWriter(&buf)
	t.Cleanup(func() { SetDefaultWriter(os.Stdout) })
	logger := New()
	logger.Printf("captured")
	if !strings.Contains(buf.String(), "captured") {
		t.Fatalf("expected log output to be written to buffer, got %q", buf.String())
	}
}

func TestAsStdLoggerReturnsUnderlyingLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithWriter(&buf)
	std := AsStdLogger(logger)
	if std == nil {
		t.Fatalf("expected std logger")
	}
	std.Print("testing")
	if !bytes.Contains(buf.Bytes(), []byte("testing")) {
		t.Fatalf("expected std logger to write to buffer")
	}
}

func TestAsStdLoggerNilSafe(t *testing.T) {
	if got := AsStdLogger(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
	var l *stdLogger
	if got := AsStdLogger(l); got != nil {
		t.Fatalf("expected nil for nil receiver")
	}
}

func TestStdLoggerPrintFSafe(t *testing.T) {
	var l *stdLogger
	l.Printf("ignore")
}

func TestStdLoggerStdLoggerSafe(t *testing.T) {
	sl := &stdLogger{}
	if sl.StdLogger() != nil {
		t.Fatalf("expected nil base")
	}
}

type captureLogger struct {
	entries []string
}

func (c *captureLogger) Printf(format string, args ...any) {
	c.entries = append(c.entries, fmt.Sprintf(format, args...))
}

func TestWithHTTPLoggingWrapsHandler(t *testing.T) {
	logger := &captureLogger{}
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("payload"))
	})
	handler := WithHTTPLogging(base, logger)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	if len(logger.entries) != 1 {
		t.Fatalf("expected single combined log entry, got %d", len(logger.entries))
	}
	var payload logPayload
	if err := json.Unmarshal([]byte(logger.entries[0]), &payload); err != nil {
		t.Fatalf("unmarshal log payload: %v", err)
	}
	if len(payload.LogEvents) != 2 {
		t.Fatalf("expected request and response entries, got %d", len(payload.LogEvents))
	}
	if payload.LogEvents[0].Direction != "request" || payload.LogEvents[1].Direction != "response" {
		t.Fatalf("expected first event to be request and second response, got %+v", payload.LogEvents)
	}
}

func TestWithHTTPLoggingUnwrapsJSONRawPayloads(t *testing.T) {
	logger := &captureLogger{}
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"message":"hi"}`))
	})
	handler := WithHTTPLogging(base, logger)

	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if len(logger.entries) != 1 {
		t.Fatalf("expected single log payload, got %d", len(logger.entries))
	}
	var payload logPayload
	if err := json.Unmarshal([]byte(logger.entries[0]), &payload); err != nil {
		t.Fatalf("unmarshal log payload: %v", err)
	}
	if len(payload.LogEvents) != 2 {
		t.Fatalf("expected request and response entries, got %d", len(payload.LogEvents))
	}
	response := payload.LogEvents[1]
	if !json.Valid(response.Raw) {
		t.Fatalf("expected raw JSON to be valid, got %q", string(response.Raw))
	}
	var decoded map[string]any
	if err := json.Unmarshal(response.Raw, &decoded); err != nil {
		t.Fatalf("expected raw JSON to decode, got error: %v (data: %q)", err, string(response.Raw))
	}
	if decoded["ok"] != true || decoded["message"] != "hi" {
		t.Fatalf("unexpected decoded raw payload: %+v", decoded)
	}
}

func TestWithHTTPLoggingOmitHTMLRawBodies(t *testing.T) {
	logger := &captureLogger{}
	base := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>hi</body></html>"))
	})
	handler := WithHTTPLogging(base, logger)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Host = "example.com"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if len(logger.entries) != 1 {
		t.Fatalf("expected single log payload, got %d", len(logger.entries))
	}
	var payload logPayload
	if err := json.Unmarshal([]byte(logger.entries[0]), &payload); err != nil {
		t.Fatalf("unmarshal log payload: %v", err)
	}
	response := payload.LogEvents[len(payload.LogEvents)-1]
	var raw string
	if err := json.Unmarshal(response.Raw, &raw); err != nil {
		t.Fatalf("expected string raw, got %v", err)
	}
	if strings.Contains(raw, "<html") {
		t.Fatalf("expected HTML body to be omitted, got %q", raw)
	}
	if !strings.Contains(raw, "http://example.com/admin") {
		t.Fatalf("expected URL reference in raw, got %q", raw)
	}
}

func TestWithHTTPLoggingNilLoggerReturnsOriginal(t *testing.T) {
	base := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if got := WithHTTPLogging(base, nil); fmt.Sprintf("%p", got) != fmt.Sprintf("%p", base) {
		t.Fatalf("expected handler to be returned untouched when logger is nil")
	}
}

func TestLoggingResponseWriterTruncatesLargeBodies(t *testing.T) {
	rr := httptest.NewRecorder()
	lrw := newLoggingResponseWriter(rr)
	payload := strings.Repeat("x", maxLoggedResponseBody+10)

	if _, err := lrw.Write([]byte(payload)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if lrw.StatusCode() != http.StatusOK {
		t.Fatalf("expected default status to be 200, got %d", lrw.StatusCode())
	}
	body := lrw.LoggedBody()
	if !strings.Contains(body, "-- response truncated after") {
		t.Fatalf("expected truncation notice, got %q", body)
	}
}

type flushRecorder struct {
	http.ResponseWriter
	flushed bool
}

func (f *flushRecorder) Flush() {
	f.flushed = true
}

func TestLoggingResponseWriterImplementsFlusher(t *testing.T) {
	fr := &flushRecorder{ResponseWriter: httptest.NewRecorder()}
	lrw := newLoggingResponseWriter(fr)
	flusher, ok := interface{}(lrw).(http.Flusher)
	if !ok {
		t.Fatalf("expected loggingResponseWriter to implement http.Flusher")
	}
	flusher.Flush()
	if !fr.flushed {
		t.Fatalf("expected underlying flusher to be invoked")
	}
}
