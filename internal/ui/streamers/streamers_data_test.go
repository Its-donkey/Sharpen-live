package streamers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

func TestMapServerStreamersOrdersOnlineFirst(t *testing.T) {
	records := []model.ServerStreamerRecord{
		{
			Streamer: model.ServerStreamerDetails{
				ID:          "offline",
				Alias:       "Offline",
				Description: "waiting",
				Languages:   []string{"English"},
			},
			Platforms: model.ServerPlatformDetails{
				YouTube: &model.ServerYouTubePlatform{Handle: "edge"},
			},
			Status: model.ServerStatus{Live: false},
		},
		{
			Streamer: model.ServerStreamerDetails{
				ID:          "online",
				Alias:       "Online",
				Description: "live",
				Languages:   []string{"French"},
			},
			Platforms: model.ServerPlatformDetails{
				Twitch: &model.ServerTwitchPlatform{Username: "forge"},
			},
			Status: model.ServerStatus{Live: true},
		},
	}
	streamers := mapServerStreamers(records)
	if len(streamers) != 2 {
		t.Fatalf("expected 2 streamers got %d", len(streamers))
	}
	if streamers[0].ID != "online" {
		t.Fatalf("expected online streamer first got %q", streamers[0].ID)
	}
	if streamers[0].Status != "online" || streamers[1].Status != "offline" {
		t.Fatalf("unexpected statuses: %+v", streamers)
	}
	if len(streamers[0].Platforms) != 1 || streamers[0].Platforms[0].ChannelURL == "" {
		t.Fatalf("expected mapped platform for online streamer")
	}
}

func TestFetchStreamersFromUsesBase(t *testing.T) {
	t.Parallel()

	want := model.WrappedStreamers{
		Streamers: []model.Streamer{
			{
				ID:          "demo",
				Name:        "Demo",
				Description: "Example streamer",
				Status:      "online",
				StatusLabel: "Online",
				Platforms: []model.Platform{
					{Name: "Twitch", ChannelURL: "https://twitch.tv/demo"},
				},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/streamers" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	streamers, err := FetchStreamersFrom(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("FetchStreamersFrom returned error: %v", err)
	}
	if len(streamers) != 1 || streamers[0].ID != "demo" {
		t.Fatalf("unexpected streamers: %+v", streamers)
	}
}
