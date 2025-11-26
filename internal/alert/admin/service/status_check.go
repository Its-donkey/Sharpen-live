package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/alert/logging"
	youtubeapi "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/api"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

// StatusCheckResult summarises the outcome of a status refresh across all channels.
type StatusCheckResult struct {
	Checked int `json:"checked"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
	Updated int `json:"updated"`
	Failed  int `json:"failed"`
}

// StatusChecker inspects the stored roster and refreshes live status for each channel.
type StatusChecker struct {
	Streamers *streamers.Store
	Search    liveSearcher
	Logger    logging.Logger
}

type liveSearcher interface {
	LiveNow(ctx context.Context, channelID string) (youtubeapi.SearchLiveResult, error)
}

const defaultStatusCheckTimeout = 8 * time.Second

// CheckAll refreshes live status for every stored channel (YouTube only).
func (c StatusChecker) CheckAll(ctx context.Context) (StatusCheckResult, error) {
	var result StatusCheckResult
	if c.Streamers == nil {
		return result, errors.New("streamers store is not configured")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return result, err
		}
	}

	records, err := c.Streamers.List()
	if err != nil {
		return result, err
	}

	search := c.search()
	logStatus(c.Logger, "Starting status refresh for %d streamers", len(records))

	for _, record := range records {
		yt := record.Platforms.YouTube
		if yt == nil {
			continue
		}
		result.Checked++
		outcome, err := c.checkYouTube(ctx, record, *yt, search)
		if err != nil {
			result.Failed++
			logStatus(c.Logger, "Status check failed for %s: %v", record.Streamer.Alias, err)
			continue
		}
		if outcome.live {
			result.Online++
		} else {
			result.Offline++
		}
		if outcome.updated {
			result.Updated++
			logStatus(c.Logger, "Updated status for %s: live=%v video=%s", record.Streamer.Alias, outcome.live, outcome.videoID)
		}
	}

	logStatus(c.Logger, "Status refresh complete: checked=%d online=%d offline=%d updated=%d failed=%d", result.Checked, result.Online, result.Offline, result.Updated, result.Failed)
	return result, nil
}

type checkOutcome struct {
	live    bool
	updated bool
	videoID string
}

func (c StatusChecker) checkYouTube(ctx context.Context, record streamers.Record, yt streamers.YouTubePlatform, search liveSearcher) (checkOutcome, error) {
	channelID := strings.TrimSpace(yt.ChannelID)
	if channelID == "" {
		channelID = extractChannelID(yt.Topic)
	}
	if channelID == "" {
		return checkOutcome{}, errors.New("youtube channel id missing")
	}

	statusCtx, cancel := withTimeout(ctx, defaultStatusCheckTimeout)
	liveResult, err := search.LiveNow(statusCtx, channelID)
	cancel()
	if err != nil {
		return checkOutcome{}, err
	}

	currentLive := record.Status != nil && record.Status.Live
	currentVideo := ""
	if record.Status != nil && record.Status.YouTube != nil {
		currentVideo = strings.TrimSpace(record.Status.YouTube.VideoID)
	}

	if liveResult.VideoID != "" {
		if currentLive && strings.EqualFold(currentVideo, liveResult.VideoID) {
			return checkOutcome{live: true, updated: false, videoID: liveResult.VideoID}, nil
		}
		if _, err := c.Streamers.UpdateYouTubeLiveStatus(channelID, streamers.YouTubeLiveStatus{
			Live:      true,
			VideoID:   liveResult.VideoID,
			StartedAt: liveResult.StartedAt,
		}); err != nil {
			return checkOutcome{}, err
		}
		return checkOutcome{live: true, updated: true, videoID: liveResult.VideoID}, nil
	}

	if currentLive {
		if _, err := c.Streamers.UpdateYouTubeLiveStatus(channelID, streamers.YouTubeLiveStatus{Live: false}); err != nil {
			return checkOutcome{}, err
		}
		return checkOutcome{live: false, updated: true}, nil
	}

	return checkOutcome{live: false, updated: false}, nil
}

func (c StatusChecker) search() liveSearcher {
	if c.Search != nil {
		return c.Search
	}
	return youtubeapi.SearchClient{
		HTTPClient: &http.Client{Timeout: defaultStatusCheckTimeout},
	}
}

func extractChannelID(topic string) string {
	if topic == "" {
		return ""
	}
	u, err := url.Parse(topic)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Query().Get("channel_id"))
}

func withTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), d)
	}
	return context.WithTimeout(parent, d)
}

func logStatus(logger logging.Logger, format string, args ...any) {
	if logger == nil {
		return
	}
	logging.LogWithID(logger, "investigative", "", fmt.Sprintf(format, args...))
}
