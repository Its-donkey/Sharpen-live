// file name — /internal/alert/logging/category_writers.go
package logging

import (
	"bufio"
	"encoding/json"
	"io"
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

// WriteCloser mirrors io.WriteCloser locally.
type WriteCloser interface {
	Write([]byte) (int, error)
	Close() error
}

// -------- NDJSON writer (unchanged) --------

func NewNDJSONWriter(file *os.File) WriteCloser {
	return &ndjsonWriter{w: bufio.NewWriter(file)}
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

// -------- Single-envelope JSON file writer (opens/closes every Write) --------

// On disk:
//
//	{"logevents":[ {...}, {...}, ... ]}
type logFileEnvelope struct {
	Logevents []json.RawMessage `json:"logevents"`
}

// What WithHTTPLogging is currently writing to the writer:
//
//	{"logevents":[ {...}, {...} ]}
type logBatch struct {
	Logevents []json.RawMessage `json:"logevents"`
}

// fileEnvelopeWriter stores just the path; it opens/closes the file for every Write.
type envelopeWriter struct {
	mu   sync.Mutex
	path string
}

// NewCategoryLogFileWriter aggregates *all* logevents into a single JSON envelope
// in the file at file.Name(). The underlying *os.File is closed immediately;
// each Write re-opens, updates, and closes the file so external processors always
// see complete JSON and the file handle is not held open.
func NewCategoryLogFileWriter(file *os.File) WriteCloser {
	if file == nil {
		return nopWriteCloser{}
	}
	path := file.Name()
	_ = file.Close()

	return &envelopeWriter{
		path: path,
	}
}

func (w *envelopeWriter) Write(p []byte) (int, error) {
	if w == nil || w.path == "" {
		return len(p), nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Determine the new events to append.
	var batch logBatch
	var newEvents []json.RawMessage

	if err := json.Unmarshal(p, &batch); err == nil && len(batch.Logevents) > 0 {
		// Normal case: incoming payload is {"logevents":[...]}.
		newEvents = batch.Logevents
	} else {
		// Fallback: treat the entire payload as a single event object.
		newEvents = []json.RawMessage{json.RawMessage(append([]byte(nil), p...))}
	}

	// Open (or create) the file for read/write.
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Read existing envelope, if any.
	var env logFileEnvelope
	data, err := io.ReadAll(f)
	if err != nil {
		return 0, err
	}
	if len(data) > 0 {
		// If this fails, we just start from an empty envelope.
		_ = json.Unmarshal(data, &env)
	}

	// Append new events.
	env.Logevents = append(env.Logevents, newEvents...)

	// Rewrite the file from scratch with the updated envelope.
	if _, err := f.Seek(0, 0); err != nil {
		return 0, err
	}
	if err := f.Truncate(0); err != nil {
		return 0, err
	}

	encoded, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return 0, err
	}
	if _, err := f.Write(encoded); err != nil {
		return 0, err
	}

	// At this point the file contains complete JSON and is closed
	// by the deferred f.Close() when Write returns.
	return len(p), nil
}

func (w *envelopeWriter) Close() error {
	// Nothing to do here: files are opened/closed per Write.
	return nil
}

type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }
