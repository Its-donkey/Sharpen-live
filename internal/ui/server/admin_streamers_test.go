package server

import (
	"testing"

	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

func TestMapStreamerRecordsMapsStatusAndLiveURL(t *testing.T) {
	records := []streamers.Record{
		{
			Streamer: streamers.Streamer{ID: "one", Alias: "Alias"},
			Platforms: streamers.Platforms{
				YouTube: &streamers.YouTubePlatform{Handle: "@handle"},
			},
			Status: &streamers.Status{
				Live:      false,
				Platforms: []string{"youtube"},
				YouTube:   &streamers.YouTubeStatus{Live: true, VideoID: "abc123"},
			},
		},
		{
			Streamer:  streamers.Streamer{ID: "two", Alias: "Two"},
			Platforms: streamers.Platforms{},
		},
	}

	mapped := mapStreamerRecords(records)

	if len(mapped) != 2 {
		t.Fatalf("expected 2 streamers, got %d", len(mapped))
	}
	first := mapped[0]
	if first.Status != "online" || first.StatusLabel != "Online" {
		t.Fatalf("expected online status, got %q/%q", first.Status, first.StatusLabel)
	}
	if len(first.Platforms) != 1 || first.Platforms[0].ChannelURL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("expected live YouTube URL, got %+v", first.Platforms)
	}

	second := mapped[1]
	if second.Status != "offline" || second.StatusLabel != "Offline" {
		t.Fatalf("expected offline status for missing status block, got %q/%q", second.Status, second.StatusLabel)
	}
}
