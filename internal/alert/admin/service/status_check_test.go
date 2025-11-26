package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	youtubeapi "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/api"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/liveinfo"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

func TestStatusCheckerUpdatesLiveState(t *testing.T) {
	store := streamers.NewStore(t.TempDir() + "/streamers.json")
	_, err := store.Append(streamers.Record{
		Streamer: streamers.Streamer{ID: "demo", Alias: "Demo"},
		Platforms: streamers.Platforms{
			YouTube: &streamers.YouTubePlatform{
				ChannelID: "UCdemo",
				Topic:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCdemo",
			},
		},
	})
	if err != nil {
		t.Fatalf("append streamer: %v", err)
	}

	feedBody := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
 <entry><yt:videoId>live123</yt:videoId></entry>
</feed>`

	checker := StatusChecker{
		Streamers: store,
		Player: stubPlayer{
			responses: map[string]youtubeapi.LiveStatus{
				"live123": {
					VideoID:           "live123",
					ChannelID:         "UCdemo",
					IsLive:            true,
					IsLiveNow:         true,
					PlayabilityStatus: "OK",
					StartedAt:         time.Date(2024, time.January, 1, 10, 0, 0, 0, time.UTC),
				},
			},
		},
		FeedClient: &http.Client{Transport: staticResponder{status: http.StatusOK, body: feedBody}},
	}

	result, err := checker.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("check all: %v", err)
	}
	if result.Checked != 1 || result.Online != 1 || result.Updated != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	records, err := store.List()
	if err != nil {
		t.Fatalf("list streamers: %v", err)
	}
	if len(records) != 1 || records[0].Status == nil || records[0].Status.YouTube == nil || !records[0].Status.YouTube.Live {
		t.Fatalf("expected live youtube status: %+v", records[0].Status)
	}
	if records[0].Status.YouTube.VideoID != "live123" {
		t.Fatalf("expected video id live123, got %q", records[0].Status.YouTube.VideoID)
	}
}

func TestStatusCheckerClearsOfflineState(t *testing.T) {
	store := streamers.NewStore(t.TempDir() + "/streamers.json")
	_, err := store.Append(streamers.Record{
		Streamer: streamers.Streamer{ID: "demo", Alias: "Demo"},
		Platforms: streamers.Platforms{
			YouTube: &streamers.YouTubePlatform{ChannelID: "UCdemo"},
		},
		Status: &streamers.Status{
			Live:      true,
			Platforms: []string{"youtube"},
			YouTube:   &streamers.YouTubeStatus{Live: true, VideoID: "live123"},
		},
	})
	if err != nil {
		t.Fatalf("append streamer: %v", err)
	}

	checker := StatusChecker{
		Streamers: store,
		Player: stubPlayer{
			responses: map[string]youtubeapi.LiveStatus{
				"live123": {VideoID: "live123", ChannelID: "UCdemo"},
			},
		},
	}

	result, err := checker.CheckAll(context.Background())
	if err != nil {
		t.Fatalf("check all: %v", err)
	}
	if result.Checked != 1 || result.Offline != 1 || result.Updated != 1 || result.Failed != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	records, err := store.List()
	if err != nil {
		t.Fatalf("list streamers: %v", err)
	}
	if records[0].Status == nil || records[0].Status.Live || (records[0].Status.YouTube != nil && records[0].Status.YouTube.Live) {
		t.Fatalf("expected youtube status cleared: %+v", records[0].Status)
	}
	if records[0].Status.YouTube != nil && records[0].Status.YouTube.VideoID != "" {
		t.Fatalf("expected video id cleared, got %q", records[0].Status.YouTube.VideoID)
	}
}

func TestStatusCheckerFallsBackToLiveInfo(t *testing.T) {
	store := streamers.NewStore(t.TempDir() + "/streamers.json")
	_, err := store.Append(streamers.Record{
		Streamer: streamers.Streamer{ID: "demo", Alias: "Demo"},
		Platforms: streamers.Platforms{
			YouTube: &streamers.YouTubePlatform{
				ChannelID: "UCdemo",
				Topic:     "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCdemo",
			},
		},
	})
	if err != nil {
		t.Fatalf("append streamer: %v", err)
	}

	feedBody := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
 <entry><yt:videoId>live456</yt:videoId></entry>
</feed>`

	checker := StatusChecker{
		Streamers:  store,
		Player:     stubPlayer{responses: map[string]youtubeapi.LiveStatus{"live456": {VideoID: "live456", ChannelID: "UCdemo"}}},
		FeedClient: &http.Client{Transport: staticResponder{status: http.StatusOK, body: feedBody}},
		LiveInfo: stubLiveInfo{
			data: map[string]youtubeapi.LiveStatus{
				"live456": {
					VideoID:           "live456",
					ChannelID:         "UCdemo",
					IsLive:            true,
					IsLiveNow:         true,
					PlayabilityStatus: "OK",
					StartedAt:         time.Date(2024, 2, 2, 15, 0, 0, 0, time.UTC),
				},
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
	records, err := store.List()
	if err != nil {
		t.Fatalf("list streamers: %v", err)
	}
	if records[0].Status == nil || records[0].Status.YouTube == nil || !records[0].Status.YouTube.Live {
		t.Fatalf("expected live status: %+v", records[0].Status)
	}
	if records[0].Status.YouTube.VideoID != "live456" {
		t.Fatalf("expected video id live456, got %q", records[0].Status.YouTube.VideoID)
	}
	if records[0].Status.YouTube.StartedAt.IsZero() {
		t.Fatalf("expected startedAt to be set")
	}
}

type stubPlayer struct {
	responses map[string]youtubeapi.LiveStatus
	err       error
}

func (p stubPlayer) LiveStatus(_ context.Context, videoID string) (youtubeapi.LiveStatus, error) {
	if resp, ok := p.responses[videoID]; ok {
		return resp, nil
	}
	if p.err != nil {
		return youtubeapi.LiveStatus{}, p.err
	}
	return youtubeapi.LiveStatus{}, fmt.Errorf("unknown video %s", videoID)
}

type staticResponder struct {
	status int
	body   string
}

func (s staticResponder) RoundTrip(_ *http.Request) (*http.Response, error) {
	statusText := http.StatusText(s.status)
	if statusText == "" {
		statusText = "Status"
	}
	return &http.Response{
		StatusCode: s.status,
		Status:     fmt.Sprintf("%d %s", s.status, statusText),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(s.body)),
	}, nil
}

type stubLiveInfo struct {
	data map[string]youtubeapi.LiveStatus
	err  error
}

func (s stubLiveInfo) Fetch(ctx context.Context, videoIDs []string) (map[string]liveinfo.VideoInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make(map[string]liveinfo.VideoInfo)
	for _, id := range videoIDs {
		if resp, ok := s.data[id]; ok {
			result[id] = liveinfo.VideoInfo{
				ID:                   resp.VideoID,
				ChannelID:            resp.ChannelID,
				Title:                resp.Title,
				LiveBroadcastContent: "live",
				ActualStartTime:      resp.StartedAt,
			}
		}
	}
	return result, nil
}
