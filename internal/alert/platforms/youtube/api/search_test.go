package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestLiveNowBuildsExpectedRequest(t *testing.T) {
	t.Helper()

	var captured *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"id":{"videoId":"abc123"},"snippet":{"publishedAt":"2024-01-02T03:04:05Z","channelId":"chan"}}]}`))
	}))
	defer server.Close()

	client := SearchClient{
		APIKey:     "test-key",
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
	}

	result, err := client.LiveNow(context.Background(), "UCFSlI8Y3Zdoq5buNW_40AAA")
	if err != nil {
		t.Fatalf("LiveNow returned error: %v", err)
	}
	if result.VideoID != "abc123" || result.ChannelID != "chan" || !result.StartedAt.Equal(time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("unexpected result: %+v", result)
	}

	if captured == nil {
		t.Fatalf("request was not captured")
	}

	u, _ := url.Parse(captured.URL.String())
	if u.Path != "/" {
		t.Fatalf("expected request path '/', got %q", u.Path)
	}
	q := u.Query()
	if got := q.Get("part"); got != "snippet" {
		t.Fatalf("part = %q, want snippet", got)
	}
	if got := q.Get("channelId"); got != "UCFSlI8Y3Zdoq5buNW_40AAA" {
		t.Fatalf("channelId = %q, want channel id", got)
	}
	if got := q.Get("eventType"); got != "live" {
		t.Fatalf("eventType = %q, want live", got)
	}
	if got := q.Get("type"); got != "video" {
		t.Fatalf("type = %q, want video", got)
	}
	if got := q.Get("key"); got != "test-key" {
		t.Fatalf("key = %q, want test-key", got)
	}
}
