package service

import (
	"bytes"
	"context"
	"errors"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/liveinfo"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	"path/filepath"
	"testing"
	"time"
)

type stubVideoLookup struct {
	infos map[string]liveinfo.VideoInfo
	err   error
}

func (s *stubVideoLookup) Fetch(ctx context.Context, videoIDs []string) (map[string]liveinfo.VideoInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.infos, nil
}

func TestAlertProcessorUpdatesLiveStatus(t *testing.T) {
	store := streamers.NewStore(filepath.Join(t.TempDir(), "streamers.json"))
	if _, err := store.Append(streamers.Record{
		Streamer: streamers.Streamer{Alias: "Demo"},
		Platforms: streamers.Platforms{
			YouTube: &streamers.YouTubePlatform{ChannelID: "UCdemo"},
		},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}
	body := `<?xml version='1.0' encoding='UTF-8'?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
 <entry>
  <yt:videoId>fbfHCxvsny0</yt:videoId>
  <yt:channelId>UCdemo</yt:channelId>
  <title>Testing 1234</title>
  <updated>2025-11-16T09:02:41+00:00</updated>
 </entry>
</feed>`
	started := time.Date(2025, 11, 16, 9, 2, 41, 0, time.UTC)
	processor := AlertProcessor{
		Streamers: store,
		VideoLookup: &stubVideoLookup{
			infos: map[string]liveinfo.VideoInfo{
				"fbfHCxvsny0": {
					ID:                   "fbfHCxvsny0",
					ChannelID:            "UCdemo",
					LiveBroadcastContent: "live",
					Live:                 true,
					ActualStartTime:      started,
				},
			},
		},
	}
	result, err := processor.Process(context.Background(), AlertProcessRequest{
		Feed: bytes.NewBufferString(body),
	})
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(result.LiveUpdates) != 1 {
		t.Fatalf("expected one live update, got %+v", result.LiveUpdates)
	}
	records, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 || !records[0].Status.Live {
		t.Fatalf("expected store live status to be set")
	}
}

func TestAlertProcessorHandlesInvalidFeed(t *testing.T) {
	processor := AlertProcessor{
		Streamers:   streamers.NewStore(filepath.Join(t.TempDir(), "streamers.json")),
		VideoLookup: &stubVideoLookup{},
	}
	_, err := processor.Process(context.Background(), AlertProcessRequest{
		Feed: bytes.NewBufferString("not xml"),
	})
	if !errors.Is(err, ErrInvalidFeed) {
		t.Fatalf("expected invalid feed error, got %v", err)
	}
}

func TestAlertProcessorHandlesLookupFailure(t *testing.T) {
	processor := AlertProcessor{
		Streamers: streamers.NewStore(filepath.Join(t.TempDir(), "streamers.json")),
		VideoLookup: &stubVideoLookup{
			err: errors.New("boom"),
		},
	}
	body := `<?xml version='1.0' encoding='UTF-8'?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
 <entry>
  <yt:videoId>abc123</yt:videoId>
  <yt:channelId>UCdemo</yt:channelId>
 </entry>
</feed>`
	_, err := processor.Process(context.Background(), AlertProcessRequest{
		Feed: bytes.NewBufferString(body),
	})
	if !errors.Is(err, ErrLookupFailed) {
		t.Fatalf("expected lookup error, got %v", err)
	}
}

func TestAlertProcessorClearsOfflineStatus(t *testing.T) {
	store := streamers.NewStore(filepath.Join(t.TempDir(), "streamers.json"))
	if _, err := store.Append(streamers.Record{
		Streamer: streamers.Streamer{Alias: "Demo"},
		Platforms: streamers.Platforms{
			YouTube: &streamers.YouTubePlatform{ChannelID: "UCdemo"},
		},
		Status: &streamers.Status{
			Live: true,
			YouTube: &streamers.YouTubeStatus{
				Live:    true,
				VideoID: "fbfHCxvsny0",
			},
			Platforms: []string{"youtube"},
		},
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	body := `<?xml version='1.0' encoding='UTF-8'?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
 <entry>
  <yt:videoId>fbfHCxvsny0</yt:videoId>
  <yt:channelId>UCdemo</yt:channelId>
  <title>Testing 1234</title>
  <updated>2025-11-16T09:10:00+00:00</updated>
 </entry>
</feed>`

	processor := AlertProcessor{
		Streamers: store,
		VideoLookup: &stubVideoLookup{
			infos: map[string]liveinfo.VideoInfo{
				"fbfHCxvsny0": {
					ID:        "fbfHCxvsny0",
					ChannelID: "UCdemo",
					Live:      false,
					IsLiveNow: false,
				},
			},
		},
	}

	result, err := processor.Process(context.Background(), AlertProcessRequest{
		Feed: bytes.NewBufferString(body),
	})
	if err != nil {
		t.Fatalf("process: %v", err)
	}
	if len(result.Offline) != 1 || result.Offline[0].ChannelID != "UCdemo" {
		t.Fatalf("expected offline update, got %+v", result.Offline)
	}
	records, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record")
	}
	status := records[0].Status
	if status == nil || status.Live || (status.YouTube != nil && status.YouTube.Live) {
		t.Fatalf("expected status to be cleared, got %+v", status)
	}
}
