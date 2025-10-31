package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Its-donkey/Sharpen-live/platforms/youtube/internal/alerts"
)

type stubProcessor struct {
	alert alerts.StreamAlert
	err   error
}

func (s *stubProcessor) Handle(ctx context.Context, alert alerts.StreamAlert) error {
	s.alert = alert
	return s.err
}

func TestHandleAlertsSuccess(t *testing.T) {
	processor := &stubProcessor{}
	srv := New(Config{Processor: processor})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"channelId":"abc","status":"online"}`))
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", rr.Code)
	}

	if processor.alert.ChannelID != "abc" {
		t.Fatalf("expected processor to receive alert")
	}
}

func TestHandleAlertsValidation(t *testing.T) {
	processor := &stubProcessor{err: alerts.ErrMissingChannelID}
	srv := New(Config{Processor: processor})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"status":"online"}`))
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleAlertsMethodNotAllowed(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleAlertsProcessorError(t *testing.T) {
	processor := &stubProcessor{err: errors.New("boom")}
	srv := New(Config{Processor: processor})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"channelId":"abc","status":"online"}`))
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}
