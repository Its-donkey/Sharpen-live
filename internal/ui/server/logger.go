package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxLogEvents = 500

type siteLogger struct {
	site      string
	general   *jsonLogFile
	http      *jsonLogFile
	websub    *jsonLogFile
	stdout    *log.Logger
	now       func() time.Time
	timefield string
}

type jsonLogFile struct {
	path string
	mu   sync.Mutex
}

type generalLogEvent struct {
	Time    string `json:"time"`
	Message string `json:"message"`
	Site    string `json:"site,omitempty"`
}

type httpLogEvent struct {
	Time       string `json:"time"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	Remote     string `json:"remote,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Site       string `json:"site,omitempty"`
}

type webSubLogEvent struct {
	Time    string `json:"time"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Query   string `json:"query,omitempty"`
	Agent   string `json:"userAgent,omitempty"`
	From    string `json:"from,omitempty"`
	Site    string `json:"site,omitempty"`
	Outcome string `json:"outcome,omitempty"`
}

func newSiteLogger(logDir, site string) (*siteLogger, error) {
	logDir = strings.TrimSpace(logDir)
	if logDir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	makeFile := func(name string) *jsonLogFile {
		return &jsonLogFile{path: filepath.Join(logDir, name)}
	}
	return &siteLogger{
		site:      site,
		general:   makeFile("general.json"),
		http:      makeFile("http.json"),
		websub:    makeFile("websub.json"),
		stdout:    log.New(os.Stdout, "", log.LstdFlags),
		now:       time.Now,
		timefield: time.RFC3339Nano,
	}, nil
}

func (l *siteLogger) Generalf(format string, args ...interface{}) {
	if l == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.stdout.Print(msg)
	_ = l.general.append(generalLogEvent{
		Time:    l.now().UTC().Format(l.timefield),
		Message: msg,
		Site:    l.site,
	})
}

func (l *siteLogger) RecordHTTP(r *http.Request, status int, duration time.Duration) {
	if l == nil {
		return
	}
	event := httpLogEvent{
		Time:       l.now().UTC().Format(l.timefield),
		Method:     r.Method,
		Path:       r.URL.Path,
		Status:     status,
		Remote:     stripHostPort(r.RemoteAddr),
		DurationMS: duration.Milliseconds(),
		Site:       l.site,
	}
	_ = l.http.append(event)
}

func (l *siteLogger) RecordWebSub(r *http.Request, outcome string) {
	if l == nil {
		return
	}
	event := webSubLogEvent{
		Time:    l.now().UTC().Format(l.timefield),
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   r.URL.RawQuery,
		Agent:   r.UserAgent(),
		From:    r.Header.Get("From"),
		Site:    l.site,
		Outcome: outcome,
	}
	_ = l.websub.append(event)
}

func stripHostPort(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		return value
	}
	return host
}

func (f *jsonLogFile) append(event interface{}) error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	payload := struct {
		LogEvents []json.RawMessage `json:"logevents"`
	}{}

	if data, err := os.ReadFile(f.path); err == nil {
		_ = json.Unmarshal(data, &payload)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	payload.LogEvents = append(payload.LogEvents, raw)
	if len(payload.LogEvents) > maxLogEvents {
		payload.LogEvents = payload.LogEvents[len(payload.LogEvents)-maxLogEvents:]
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0o644)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.maybeSetStatus()
	return r.ResponseWriter.Write(b)
}

func (s *server) withHTTPLogging(next http.Handler) http.Handler {
	if s == nil || s.logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)
		s.logger.RecordHTTP(r, rec.status, time.Since(start))
	})
}

func (s *server) logf(format string, args ...interface{}) {
	if s != nil && s.logger != nil {
		s.logger.Generalf(format, args...)
		return
	}
	log.Printf(format, args...)
}

func (r *statusRecorder) maybeSetStatus() {
	if r.wrote {
		return
	}
	r.wrote = true
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.ResponseWriter.WriteHeader(r.status)
}
