// Package logging provides structured logging with multiple output formats and levels.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents the severity of a log entry.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Entry represents a single log entry with structured fields.
type Entry struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     string         `json:"level"`
	Category  string         `json:"category"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
	Duration  *int64         `json:"duration_ms,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// Logger is a structured logger that writes to multiple outputs.
type Logger struct {
	mu          sync.RWMutex
	minLevel    Level
	writers     []io.Writer
	site        string
	subscribers []chan<- Entry
}

// Global registry for all loggers
var (
	registryMu sync.RWMutex
	registry   = make(map[string]*Logger)
)

// New creates a new Logger instance and registers it in the global registry.
func New(site string, minLevel Level, writers ...io.Writer) *Logger {
	l := &Logger{
		minLevel:    minLevel,
		writers:     writers,
		site:        site,
		subscribers: make([]chan<- Entry, 0),
	}

	// Register logger in global registry
	registryMu.Lock()
	registry[site] = l
	registryMu.Unlock()

	return l
}

// AllLoggers returns all registered loggers.
func AllLoggers() []*Logger {
	registryMu.RLock()
	defer registryMu.RUnlock()

	loggers := make([]*Logger, 0, len(registry))
	for _, logger := range registry {
		loggers = append(loggers, logger)
	}
	return loggers
}

// Subscribe adds a channel to receive log entries in real-time.
func (l *Logger) Subscribe(ch chan<- Entry) func() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.subscribers = append(l.subscribers, ch)

	// Return unsubscribe function
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		for i, sub := range l.subscribers {
			if sub == ch {
				l.subscribers = append(l.subscribers[:i], l.subscribers[i+1:]...)
				break
			}
		}
	}
}

// Log writes a log entry at the specified level.
func (l *Logger) Log(level Level, category, message string, fields map[string]any) {
	if level < l.minLevel {
		return
	}

	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     level.String(),
		Category:  category,
		Message:   message,
		Fields:    fields,
	}

	l.write(entry)
}

// Debug logs a debug message.
func (l *Logger) Debug(category, message string, fields map[string]any) {
	l.Log(DEBUG, category, message, fields)
}

// Info logs an info message.
func (l *Logger) Info(category, message string, fields map[string]any) {
	l.Log(INFO, category, message, fields)
}

// Warn logs a warning message.
func (l *Logger) Warn(category, message string, fields map[string]any) {
	l.Log(WARN, category, message, fields)
}

// Error logs an error message.
func (l *Logger) Error(category, message string, err error, fields map[string]any) {
	if ERROR < l.minLevel {
		return
	}
	if fields == nil {
		fields = make(map[string]any)
	}
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     ERROR.String(),
		Category:  category,
		Message:   message,
		Fields:    fields,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	l.write(entry)
}

func (l *Logger) write(entry Entry) {
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal log entry: %v\n", err)
		return
	}
	data = append(data, '\n')

	l.mu.RLock()
	writers := l.writers
	subscribersCopy := make([]chan<- Entry, len(l.subscribers))
	copy(subscribersCopy, l.subscribers)
	l.mu.RUnlock()

	// Write to all configured writers
	for _, w := range writers {
		_, _ = w.Write(data)
	}

	// Send to all subscribers (non-blocking)
	for _, ch := range subscribersCopy {
		select {
		case ch <- entry:
		default:
			// Skip if channel is full
		}
	}
}

// WithRequestID returns a new logger context with a request ID.
type LogContext struct {
	logger    *Logger
	requestID string
	category  string
	fields    map[string]any
}

// WithRequestID creates a logging context with a request ID.
func (l *Logger) WithRequestID(requestID string) *LogContext {
	return &LogContext{
		logger:    l,
		requestID: requestID,
		fields:    make(map[string]any),
	}
}

// WithCategory sets the category for this context.
func (c *LogContext) WithCategory(category string) *LogContext {
	c.category = category
	return c
}

// WithField adds a field to this context.
func (c *LogContext) WithField(key string, value any) *LogContext {
	if c.fields == nil {
		c.fields = make(map[string]any)
	}
	c.fields[key] = value
	return c
}

// WithFields adds multiple fields to this context.
func (c *LogContext) WithFields(fields map[string]any) *LogContext {
	if c.fields == nil {
		c.fields = make(map[string]any)
	}
	for k, v := range fields {
		c.fields[k] = v
	}
	return c
}

// Info logs an info message with the context's request ID and fields.
func (c *LogContext) Info(message string) {
	if INFO < c.logger.minLevel {
		return
	}
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     INFO.String(),
		Category:  c.category,
		Message:   message,
		Fields:    c.fields,
		RequestID: c.requestID,
	}
	c.logger.write(entry)
}

// Error logs an error message with the context's request ID and fields.
func (c *LogContext) Error(message string, err error) {
	if ERROR < c.logger.minLevel {
		return
	}
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     ERROR.String(),
		Category:  c.category,
		Message:   message,
		Fields:    c.fields,
		RequestID: c.requestID,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	c.logger.write(entry)
}

// Warn logs a warning message with the context's request ID and fields.
func (c *LogContext) Warn(message string) {
	if WARN < c.logger.minLevel {
		return
	}
	entry := Entry{
		Timestamp: time.Now().UTC(),
		Level:     WARN.String(),
		Category:  c.category,
		Message:   message,
		Fields:    c.fields,
		RequestID: c.requestID,
	}
	c.logger.write(entry)
}
