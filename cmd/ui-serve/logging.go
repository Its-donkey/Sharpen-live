//go:build !js && !wasm

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type serverLogger interface {
	Printf(format string, v ...any)
	Log(entry logEntry)
}

const (
	maxLoggedResponseBody = 4096
	maxStoredLogEntries   = 300
)

var logStream = newLogStream()

func newServerLogger() serverLogger {
	return newServerLoggerWithWriter(os.Stdout)
}

func newServerLoggerWithWriter(w io.Writer) serverLogger {
	if w == nil {
		w = os.Stdout
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	return &jsonLogger{enc: enc}
}

type jsonLogger struct {
	mu  sync.Mutex
	enc *json.Encoder
}

type logEntry map[string]any

func (l *jsonLogger) Printf(format string, v ...any) {
	l.Log(logEntry{"message": fmt.Sprintf(format, v...)})
}

func (l *jsonLogger) Log(entry logEntry) {
	if l == nil || l.enc == nil || entry == nil {
		return
	}
	if _, ok := entry["time"]; !ok {
		if _, has := entry["datetime"]; !has {
			entry["time"] = time.Now().UTC().Format(time.RFC3339Nano)
		}
	}
	l.mu.Lock()
	err := l.enc.Encode(entry)
	l.mu.Unlock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "json logger encode error: %v\n", err)
	}
	logStream.Broadcast(entry)
}

func withHTTPLogging(next http.Handler, logger serverLogger) http.Handler {
	if next == nil || logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqEntry := logEntry{
			"datetime":     time.Now().UTC().Format(time.RFC3339Nano),
			"message":      fmt.Sprintf("---- Incoming request from %s ----", r.RemoteAddr),
			"http-request": fmt.Sprintf("%s %s %s", r.Method, requestTarget(r), r.Proto),
		}
		if host := r.Host; host != "" {
			reqEntry["Host"] = host
		}
		for name, vals := range r.Header {
			reqEntry[name] = strings.Join(vals, ", ")
		}
		logger.Log(reqEntry)

		lrw := newLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)

		status := lrw.StatusCode()
		respEntry := logEntry{
			"datetime":            time.Now().UTC().Format(time.RFC3339Nano),
			"message":             fmt.Sprintf("---- Response for %s %s ----", r.Method, r.URL.Path),
			"http-response":       fmt.Sprintf("%d %s", status, http.StatusText(status)),
			"response-bytes":      lrw.BytesWritten(),
			"response-body":       lrw.LoggedBody(),
			"response-body-trunc": lrw.Truncated(),
		}
		logger.Log(respEntry)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status    int
	buf       bytes.Buffer
	truncated bool
	written   int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{ResponseWriter: w}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.status = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(b)
	if lrw.status == 0 {
		lrw.status = http.StatusOK
	}
	lrw.written += n
	if lrw.buf.Len() < maxLoggedResponseBody {
		remaining := maxLoggedResponseBody - lrw.buf.Len()
		chunk := n
		if chunk > remaining {
			chunk = remaining
			lrw.truncated = true
		}
		if chunk > 0 {
			lrw.buf.Write(b[:chunk])
		}
		if n > chunk {
			lrw.truncated = true
		}
	} else if n > 0 {
		lrw.truncated = true
	}
	return n, err
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

func (lrw *loggingResponseWriter) Truncated() bool {
	return lrw.truncated
}

func (lrw *loggingResponseWriter) BytesWritten() int {
	return lrw.written
}

func (lrw *loggingResponseWriter) Flush() {
	if flusher, ok := lrw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func requestTarget(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "/"
	}
	if uri := r.URL.RequestURI(); uri != "" {
		return uri
	}
	if path := r.URL.Path; path != "" {
		return path
	}
	return "/"
}

type logStreamState struct {
	mu          sync.RWMutex
	buffer      [][]byte
	subscribers map[chan []byte]struct{}
}

func newLogStream() *logStreamState {
	return &logStreamState{
		subscribers: make(map[chan []byte]struct{}),
	}
}

func (h *logStreamState) Broadcast(entry logEntry) {
	if h == nil || entry == nil {
		return
	}
	data, err := json.MarshalIndent(entry, "", "\t")
	if err != nil {
		return
	}
	h.mu.Lock()
	if len(h.buffer) >= maxStoredLogEntries {
		copy(h.buffer, h.buffer[1:])
		h.buffer[len(h.buffer)-1] = append([]byte(nil), data...)
	} else {
		h.buffer = append(h.buffer, append([]byte(nil), data...))
	}
	for ch := range h.subscribers {
		select {
		case ch <- data:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *logStreamState) Subscribe() (chan []byte, [][]byte) {
	ch := make(chan []byte, 64)
	h.mu.RLock()
	snapshot := make([][]byte, len(h.buffer))
	for i := range h.buffer {
		snapshot[i] = append([]byte(nil), h.buffer[i]...)
	}
	h.mu.RUnlock()

	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch, snapshot
}

func (h *logStreamState) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	if _, ok := h.subscribers[ch]; ok {
		delete(h.subscribers, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func logsStreamHandler(api *url.URL) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" || !validateAdminToken(r.Context(), api, token) {
			http.Error(w, "admin authentication required", http.StatusUnauthorized)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch, snapshot := logStream.Subscribe()
		defer logStream.Unsubscribe(ch)

		for _, entry := range snapshot {
			writeSSEPayload(w, entry)
		}
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case entry, ok := <-ch:
				if !ok {
					return
				}
				writeSSEPayload(w, entry)
				flusher.Flush()
			}
		}
	})
}

func writeSSEPayload(w io.Writer, data []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 4096)
	scanner.Buffer(buf, 1<<20)
	for scanner.Scan() {
		fmt.Fprintf(w, "data: %s\n", scanner.Text())
	}
	fmt.Fprint(w, "\n")
}

func validateAdminToken(parent context.Context, api *url.URL, token string) bool {
	if api == nil || strings.TrimSpace(token) == "" {
		return false
	}
	paths := []string{
		"/api/admin/settings",
		"/api/admin/submissions",
	}
	for _, p := range paths {
		if probeAdminEndpoint(parent, api, token, p) {
			return true
		}
	}
	return false
}

func probeAdminEndpoint(parent context.Context, api *url.URL, token, path string) bool {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	target := api.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
