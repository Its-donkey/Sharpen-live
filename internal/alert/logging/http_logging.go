package logging

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/google/uuid"
)

const maxLoggedResponseBody = 4096

type requestIDKey struct{}

var contextRequestIDKey requestIDKey

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
				Category:  "http",
				Direction: "request",
				Message:   fmt.Sprintf("Incoming request from %s", r.RemoteAddr),
				Raw:       encodeLogRaw(string(dump)),
				Method:    r.Method,
				Path:      r.URL.Path,
				Query:     r.URL.RawQuery,
				Host:      r.Host,
				Proto:     r.Proto,
				UserAgent: r.UserAgent(),
				Referer:   r.Referer(),
				Remote:    r.RemoteAddr,
			})
		} else {
			logger.Printf("failed to dump request from %s: %v", r.RemoteAddr, err)
		}

		// Share the request ID with downstream handlers via context so general logs can correlate.
		ctxWithID := context.WithValue(r.Context(), contextRequestIDKey, requestID)
		r = r.WithContext(ctxWithID)

		lrw := newLoggingResponseWriter(w)
		start := time.Now()
		defer func() {
			status := lrw.StatusCode()
			responseEvent := logEvent{
				Time:      time.Now().UTC().Format(time.RFC3339Nano),
				ID:        requestID,
				Category:  "http",
				Direction: "response",
				Message:   fmt.Sprintf("Response for %s %s (%d %s)", r.Method, r.URL.Path, status, http.StatusText(status)),
				Raw:       encodeLogRaw(lrw.LoggedBody()),
				Method:    r.Method,
				Path:      r.URL.Path,
				Query:     r.URL.RawQuery,
				Host:      r.Host,
				Proto:     r.Proto,
				UserAgent: r.UserAgent(),
				Referer:   r.Referer(),
				Status:    status,
				Remote:    r.RemoteAddr,
				Response:  lrw.BytesWritten(),
				Duration:  time.Since(start).Milliseconds(),
			}
			events = append(events, responseEvent)
			logJSON(logger, events...)
		}()

		next.ServeHTTP(lrw, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status       int
	buf          bytes.Buffer
	truncated    bool
	bytesWritten int64
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
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytesWritten += int64(n)
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

func (lrw *loggingResponseWriter) Flush() {
	if flusher, ok := lrw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (lrw *loggingResponseWriter) BytesWritten() int64 {
	return lrw.bytesWritten
}

// RequestIDFromContext extracts the request ID stored by WithHTTPLogging so
// other logs can correlate with HTTP logs.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(contextRequestIDKey).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
