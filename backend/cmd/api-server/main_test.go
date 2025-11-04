package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSpaHandlerServesExistingFile(t *testing.T) {
	tmp := t.TempDir()
	aboutPath := filepath.Join(tmp, "about.html")
	if err := os.WriteFile(aboutPath, []byte("about"), 0o644); err != nil {
		t.Fatalf("write about file: %v", err)
	}
	indexPath := filepath.Join(tmp, "index.html")
	if err := os.WriteFile(indexPath, []byte("index"), 0o644); err != nil {
		t.Fatalf("write index file: %v", err)
	}

	handler := spaHandler(tmp)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/about.html", nil)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "about" {
		t.Fatalf("expected about body, got %q", rr.Body.String())
	}
}

func TestSpaHandlerFallsBackToIndex(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatalf("write index file: %v", err)
	}

	handler := spaHandler(tmp)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 fallback, got %d", rr.Code)
	}
	if rr.Body.String() != "index" {
		t.Fatalf("expected index content, got %q", rr.Body.String())
	}
}

func TestSpaHandlerMissingIndex(t *testing.T) {
	tmp := t.TempDir()
	handler := spaHandler(tmp)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when index missing, got %d", rr.Code)
	}
}
