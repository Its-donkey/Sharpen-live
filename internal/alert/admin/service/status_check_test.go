package service

import (
	"context"
	youtubeapi "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/api"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	"testing"
	"time"
)

func TestStatusCheckerMarksLiveFromSearch(t *testing.T) {
	store := streamers.NewStore(t.TempDir() + "/streamers.json")
	_, err := store.Append(streamers.Record{
		Streamer: streamers.Streamer{ID: "demo", Alias: "Demo"},
		Platforms: streamers.Platforms{
			YouTube: &streamers.YouTubePlatform{ChannelID: "UCdemo"},
		},
	})
	if err != nil {
		t.Fatalf("append streamer: %v", err)
	}

	checker := StatusChecker{
		Streamers: store,
		Search: stubSearch{
			result: youtubeapi.SearchLiveResult{
				VideoID:   "live123",
				ChannelID: "UCdemo",
				StartedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			},
		},
	}

	result, err := checker.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("check all: %v", err)
	}
	if result.Online != 1 || result.Updated != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	records, _ := store.List()
	if records[0].Status == nil || records[0].Status.YouTube == nil || !records[0].Status.YouTube.Live {
		t.Fatalf("expected live status: %+v", records[0].Status)
	}
	if records[0].Status.YouTube.VideoID != "live123" {
		t.Fatalf("expected video id live123, got %q", records[0].Status.YouTube.VideoID)
	}
}

func TestStatusCheckerClearsOfflineWhenNoLive(t *testing.T) {
	store := streamers.NewStore(t.TempDir() + "/streamers.json")
	_, err := store.Append(streamers.Record{
		Streamer: streamers.Streamer{ID: "demo", Alias: "Demo"},
		Platforms: streamers.Platforms{
			YouTube: &streamers.YouTubePlatform{ChannelID: "UCdemo"},
		},
		Status: &streamers.Status{
			Live:    true,
			YouTube: &streamers.YouTubeStatus{Live: true, VideoID: "old"},
		},
	})
	if err != nil {
		t.Fatalf("append streamer: %v", err)
	}

	checker := StatusChecker{
		Streamers: store,
		Search:    stubSearch{},
	}
	result, err := checker.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("check all: %v", err)
	}
	if result.Offline != 1 || result.Updated != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	records, _ := store.List()
	if records[0].Status != nil && records[0].Status.Live {
		t.Fatalf("expected live cleared: %+v", records[0].Status)
	}
	if records[0].Status != nil && records[0].Status.YouTube != nil && records[0].Status.YouTube.VideoID != "" {
		t.Fatalf("expected video id cleared, got %q", records[0].Status.YouTube.VideoID)
	}
}

type stubSearch struct {
	result youtubeapi.SearchLiveResult
	err    error
}

func (s stubSearch) LiveNow(context.Context, string) (youtubeapi.SearchLiveResult, error) {
	if s.err != nil {
		return youtubeapi.SearchLiveResult{}, s.err
	}
	return s.result, nil
}
