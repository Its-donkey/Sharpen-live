package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddCORSForwardsRequest(t *testing.T) {
	nextCalled := false
	handler := addCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusTeapot)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	handler.ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatalf("expected downstream handler to be invoked")
	}
	if rr.Code != http.StatusTeapot {
		t.Fatalf("expected status 418, got %d", rr.Code)
	}

	headers := rr.Result().Header
	if headers.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS origin header")
	}
	if headers.Get("Access-Control-Allow-Methods") == "" {
		t.Fatalf("missing CORS methods header")
	}
	if headers.Get("Access-Control-Allow-Headers") == "" {
		t.Fatalf("missing CORS headers header")
	}
}

func TestAddCORSHandlesOptions(t *testing.T) {
	nextCalled := false
	handler := addCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)

	handler.ServeHTTP(rr, req)

	if nextCalled {
		t.Fatalf("expected OPTIONS request to short-circuit")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rr.Code)
	}
}
