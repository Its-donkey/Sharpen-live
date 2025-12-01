package logging

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// HTTPLogger logs HTTP requests and responses with full details.
type HTTPLogger struct {
	logger      *Logger
	maxBodySize int
}

// NewHTTPLogger creates a new HTTP logger.
func NewHTTPLogger(logger *Logger, maxBodySize int) *HTTPLogger {
	if maxBodySize == 0 {
		maxBodySize = 10 * 1024 // 10KB default
	}
	return &HTTPLogger{
		logger:      logger,
		maxBodySize: maxBodySize,
	}
}

// responseRecorder captures the response for logging.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	size        int
	body        *bytes.Buffer
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
		r.ResponseWriter.WriteHeader(status)
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	if r.body != nil && r.body.Len() < 10*1024 {
		r.body.Write(b[:min(len(b), 10*1024-r.body.Len())])
	}
	return n, err
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("responseRecorder does not support hijacking")
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Middleware returns an HTTP middleware that logs requests and responses.
func (h *HTTPLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := uuid.New().String()

		// Read and buffer request body
		var requestBody string
		if r.Body != nil && r.ContentLength > 0 && r.ContentLength < int64(h.maxBodySize) {
			bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, int64(h.maxBodySize)))
			if err == nil {
				requestBody = string(bodyBytes)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Create response recorder
		recorder := &responseRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
			body:           &bytes.Buffer{},
		}

		// Add request ID to response header
		recorder.Header().Set("X-Request-ID", requestID)

		// Process request
		next.ServeHTTP(recorder, r)

		duration := time.Since(start).Milliseconds()

		// Log the request/response
		fields := map[string]any{
			"method":         r.Method,
			"path":           r.URL.Path,
			"query":          r.URL.RawQuery,
			"status":         recorder.status,
			"size":           recorder.size,
			"remote_addr":    r.RemoteAddr,
			"user_agent":     r.UserAgent(),
			"referer":        r.Referer(),
			"content_type":   r.Header.Get("Content-Type"),
			"content_length": r.ContentLength,
		}

		if requestBody != "" {
			fields["request_body"] = truncate(requestBody, 1000)
		}

		if recorder.body.Len() > 0 && !strings.HasPrefix(recorder.Header().Get("Content-Type"), "image/") {
			fields["response_body"] = truncate(recorder.body.String(), 1000)
		}

		// Add request headers (excluding sensitive ones)
		headers := make(map[string]string)
		for name, values := range r.Header {
			if !isSensitiveHeader(name) {
				headers[name] = strings.Join(values, ", ")
			}
		}
		if len(headers) > 0 {
			fields["request_headers"] = headers
		}

		entry := Entry{
			Timestamp: time.Now().UTC(),
			Level:     INFO.String(),
			Category:  "http",
			Message:   fmt.Sprintf("%s %s %d", r.Method, r.URL.Path, recorder.status),
			Fields:    fields,
			RequestID: requestID,
			Duration:  &duration,
		}

		if recorder.status >= 400 {
			entry.Level = WARN.String()
		}
		if recorder.status >= 500 {
			entry.Level = ERROR.String()
		}

		h.logger.write(entry)

		// Parse YouTube WebSub notifications if detected
		if requestBody != "" && IsYouTubeWebSubNotification(requestBody, r.Header.Get("Content-Type")) {
			h.logger.ParseYouTubeWebSub(requestBody, requestID)
		}
	})
}

func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "auth") ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "cookie") ||
		strings.Contains(lower, "key") ||
		strings.Contains(lower, "secret")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
