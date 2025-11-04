package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/alerts"
)

type stubProcessor struct {
	alerts []alerts.StreamAlert
	err    error
}

func (s *stubProcessor) Handle(ctx context.Context, alert alerts.StreamAlert) error {
	s.alerts = append(s.alerts, alert)
	return s.err
}

const sampleAtomNotification = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom" xmlns:yt="http://www.youtube.com/xml/schemas/2015">
  <entry>
    <yt:videoId>video-123</yt:videoId>
    <yt:channelId>UCCHAN</yt:channelId>
    <title>Test Stream</title>
    <published>2025-11-05T07:13:36Z</published>
    <updated>2025-11-05T07:14:39Z</updated>
    <link rel="alternate" href="https://www.youtube.com/watch?v=video-123" />
  </entry>
</feed>`

func TestNotificationAtom(t *testing.T) {
	processor := &stubProcessor{}
	srv := New(Config{
		Processor: processor,
		Streamers: StreamerDirectory{"UCCHAN": "Test"},
		Immediate: true,
	})

	entries, err := parseAtomFeed([]byte(sampleAtomNotification))
	if err != nil {
		t.Fatalf("parseAtomFeed failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry from atom feed")
	}
	if entries[0].ChannelID != "UCCHAN" || entries[0].VideoID != "video-123" {
		t.Fatalf("unexpected parsed entry: %#v", entries[0])
	}

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(sampleAtomNotification))
	req.Header.Set("Content-Type", "application/atom+xml; charset=UTF-8")
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d body=%q", rr.Code, rr.Body.String())
	}

	if len(processor.alerts) != 1 {
		t.Fatalf("expected processor to receive 1 alert, got %d", len(processor.alerts))
	}

	alert := processor.alerts[0]
	if alert.ChannelID != "UCCHAN" || alert.StreamID != "video-123" || alert.Status != "online" || alert.StreamerName != "Test" {
		t.Fatalf("unexpected alert payload: %#v", alert)
	}
}

func TestNotificationAtomProcessorError(t *testing.T) {
	processor := &stubProcessor{err: errors.New("boom")}
	srv := New(Config{
		Processor: processor,
		Streamers: StreamerDirectory{"UCCHAN": "Test"},
		Immediate: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(sampleAtomNotification))
	req.Header.Set("Content-Type", "application/atom+xml")
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%q", rr.Code, rr.Body.String())
	}

	if len(processor.alerts) != 1 {
		t.Fatalf("expected alert to be delivered despite processor error")
	}
}

func TestNotificationAtomFallbackToBodyPrefix(t *testing.T) {
	processor := &stubProcessor{}
	srv := New(Config{
		Processor: processor,
		Streamers: StreamerDirectory{"UCCHAN": "Test"},
		Immediate: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(sampleAtomNotification))
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%q", rr.Code, rr.Body.String())
	}
}

func TestNotificationUnsupportedPayload(t *testing.T) {
	srv := New(Config{
		Streamers: StreamerDirectory{"UCCHAN": "Test"},
		Immediate: true,
	})

	req := httptest.NewRequest(http.MethodPost, "/alerts", strings.NewReader(`{"hello":"world"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestVerificationSuccess(t *testing.T) {
	srv := New(Config{
		Streamers: StreamerDirectory{"UCCHAN": "Test"},
	})

	topic := "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCCHAN"
	req := httptest.NewRequest(http.MethodGet, "/alerts?hub.mode=subscribe&hub.challenge=abc123&hub.topic="+topic+"&hub.lease_seconds=3600", nil)
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	if body := strings.TrimSpace(rr.Body.String()); body != "abc123" {
		t.Fatalf("expected challenge echo, got %q", body)
	}
}

func TestVerificationTopicMismatch(t *testing.T) {
	srv := New(Config{
		Streamers: StreamerDirectory{"UCCHAN": "Test"},
	})

	req := httptest.NewRequest(http.MethodGet, "/alerts?hub.mode=subscribe&hub.challenge=test&hub.topic=https://example.com/feed", nil)
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestVerificationMissingChallenge(t *testing.T) {
	srv := New(Config{
		Streamers: StreamerDirectory{"UCCHAN": "Test"},
	})

	req := httptest.NewRequest(http.MethodGet, "/alerts?hub.mode=subscribe", nil)
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUnsupportedMethod(t *testing.T) {
	srv := New(Config{
		Streamers: StreamerDirectory{"UCCHAN": "Test"},
	})

	req := httptest.NewRequest(http.MethodPut, "/alerts", nil)
	rr := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
