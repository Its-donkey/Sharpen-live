package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLiveMissingAPIKey(t *testing.T) {
	checker := NewChecker("")
	if _, err := checker.IsLive(context.Background(), "channel", ""); err == nil {
		t.Fatal("expected error for missing api key")
	}
}

func TestIsLiveStream(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "stream123" {
			t.Fatalf("expected stream ID param")
		}
		if r.URL.Query().Get("part") != "liveStreamingDetails" {
			t.Fatalf("unexpected part param")
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{
				map[string]any{
					"liveStreamingDetails": map[string]any{"actualEndTime": ""},
				},
			},
		})
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	checker := NewChecker("key", WithHTTPClient(srv.Client()), WithEndpoints(srv.URL, srv.URL))

	live, err := checker.IsLive(context.Background(), "channel", "stream123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !live {
		t.Fatal("expected stream to be live")
	}
}

func TestIsLiveChannel(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("channelId") != "channel123" {
			t.Fatalf("expected channelId param")
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []any{map[string]any{}},
		})
	}

	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)

	checker := NewChecker("key", WithHTTPClient(srv.Client()), WithEndpoints(srv.URL, srv.URL))
	live, err := checker.IsLive(context.Background(), "channel123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !live {
		t.Fatal("expected channel to be live")
	}
}

func TestIsLiveChannelMissingID(t *testing.T) {
	checker := NewChecker("key")
	if _, err := checker.IsLive(context.Background(), "", ""); err == nil {
		t.Fatal("expected error for missing channel ID")
	}
}
