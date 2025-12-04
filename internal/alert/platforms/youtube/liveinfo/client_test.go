package liveinfo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientFetchParsesLivePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/watch" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("v") == "" {
			t.Fatalf("missing video id")
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><html><head><script>var ytInitialPlayerResponse = {"videoDetails":{"videoId":"abc123","channelId":"UCdemo","title":"Live demo","isLiveContent":true,"isLive":true},"microformat":{"playerMicroformatRenderer":{"liveBroadcastDetails":{"startTimestamp":"2025-11-16T09:02:41Z","isLiveNow":true}}}};;</script></head><body></body></html>`))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		BaseURL:    server.URL + "/watch",
	}

	info, err := client.Fetch(context.Background(), []string{"abc123"})
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	entry, ok := info["abc123"]
	if !ok {
		t.Fatalf("expected entry for video")
	}
	if !entry.IsLive() {
		t.Fatalf("expected entry to be live")
	}
	if entry.ChannelID != "UCdemo" {
		t.Fatalf("unexpected channel id %q", entry.ChannelID)
	}
	if entry.ActualStartTime.IsZero() {
		t.Fatalf("expected start timestamp to be parsed")
	}
}

func TestClientFetchSkipsFailures(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "bad", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><script>var ytInitialPlayerResponse = {"videoDetails":{"videoId":"def456","channelId":"UCdemo","title":"Demo","isLiveContent":false}};</script>`))
	}))
	defer server.Close()

	client := &Client{
		HTTPClient: server.Client(),
		BaseURL:    server.URL + "/watch",
	}

	info, err := client.Fetch(context.Background(), []string{"abc123", "def456"})
	if err != nil {
		t.Fatalf("expected partial success, got error %v", err)
	}
	if len(info) != 1 {
		t.Fatalf("expected one successful entry, got %d", len(info))
	}
}

func TestVideoInfoIsLive(t *testing.T) {
	cases := []struct {
		name string
		info VideoInfo
		live bool
	}{
		{name: "live flag", info: VideoInfo{Live: true}, live: true},
		{name: "live now", info: VideoInfo{IsLiveNow: true}, live: true},
		{name: "ended stream", info: VideoInfo{LiveBroadcastContent: "live", ActualEndTime: time.Now()}, live: false},
		{name: "offline default", info: VideoInfo{}, live: false},
	}
	for _, tc := range cases {
		if tc.info.IsLive() != tc.live {
			t.Fatalf("%s: expected live=%v, got %v", tc.name, tc.live, tc.info.IsLive())
		}
	}
}
