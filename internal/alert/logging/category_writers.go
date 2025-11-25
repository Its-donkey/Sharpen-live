// file name — /internal/alert/logging/category_writers.go
package logging

import (
	"bufio"
	"encoding/json"
	"os"
	"sync"
)

var categoryWriters sync.Map

// SetCategoryWriter registers a writer that receives copies of log payloads for the given category.
// Passing a nil writer removes any existing category writer.
func SetCategoryWriter(category string, w WriteCloser) {
	if category == "" {
		return
	}
	if w == nil {
		if existing, ok := categoryWriters.LoadAndDelete(category); ok {
			if closer, ok := existing.(WriteCloser); ok {
				_ = closer.Close()
			}
		}
		return
	}
	categoryWriters.Store(category, w)
}

// NewNDJSONWriter wraps the file so each payload is written as a single line of JSON.
func NewNDJSONWriter(file *os.File) WriteCloser {
	return &ndjsonWriter{w: bufio.NewWriter(file)}
}

// NewCategoryLogFileWriter returns a writer that aggregates *all* logevents
// into a single JSON envelope:
//
//   {"logevents":[
//     {...},
//     {...},
//     ...
//   ]}
func NewCategoryLogFileWriter(file *os.File) WriteCloser {
	return &envelopeWriter{
		w:     bufio.NewWriter(file),
		first: true,
	}
}

// WriteCloser mirrors io.WriteCloser locally to avoid importing the entire interface hierarchy here.
type WriteCloser interface {
	Write([]byte) (int, error)
	Close() error
}

type ndjsonWriter struct {
	mu sync.Mutex
	w  *bufio.Writer
}

func (n *ndjsonWriter) Write(p []byte) (int, error) {
	if n == nil || n.w == nil {
		return len(p), nil
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(p) > 0 && p[len(p)-1] != '\n' {
		if _, err := n.w.Write(append(p, '\n')); err != nil {
			return 0, err
		}
	} else {
		if _, err := n.w.Write(p); err != nil {
			return 0, err
		}
	}
	if err := n.w.Flush(); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (n *ndjsonWriter) Close() error {
	if n == nil || n.w == nil {
		return nil
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.w.Flush()
}

// ---- Single-envelope category writer ----

// logBatch matches the shape that WithHTTPLogging is writing:
//
//   {"logevents":[{...},{...}]}
type logBatch struct {
	Events []json.RawMessage `json:"logevents"`
}

// envelopeWriter flattens each incoming {"logevents":[...]} batch
// into a single outer {"logevents":[ ... ]} in the file.
type envelopeWriter struct {
	mu     sync.Mutex
	w      *bufio.Writer
	opened bool // header written
	first  bool // true until first event is written
}

func (e *envelopeWriter) Write(p []byte) (int, error) {
	if e == nil || e.w == nil {
		return len(p), nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Write the outer envelope header once.
	if !e.opened {
		if _, err := e.w.WriteString(`{"logevents":[` + "\n"); err != nil {
			return 0, err
		}
		e.opened = true
	}

	// Try to parse the payload as {"logevents":[...]}.
	var batch logBatch
	if err := json.Unmarshal(p, &batch); err != nil || len(batch.Events) == 0 {
		// Fallback: treat the payload itself as a single event object.
		if !e.first {
			if _, err := e.w.WriteString(",\n"); err != nil {
				return 0, err
			}
		}
		if _, err := e.w.Write(p); err != nil {
			return 0, err
		}
		e.first = false
		return len(p), e.w.Flush()
	}

	// Normal case: flatten each event in the batch.Events slice.
	for _, ev := range batch.Events {
		if !e.first {
			if _, err := e.w.WriteString(",\n"); err != nil {
				return 0, err
			}
		}
		if _, err := e.w.Write(ev); err != nil {
			return 0, err
		}
		e.first = false
	}

	if err := e.w.Flush(); err != nil {
		return 0, err
	}

	return len(p), nil
}

func (e *envelopeWriter) Close() error {
	if e == nil || e.w == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// No writes at all? Still emit a valid empty array.
	if !e.opened {
		if _, err := e.w.WriteString(`{"logevents":[]}` + "\n"); err != nil {
			return err
		}
		return e.w.Flush()
	}

	if _, err := e.w.WriteString("\n]}\n"); err != nil {
		return err
	}
	return e.w.Flush()
}
