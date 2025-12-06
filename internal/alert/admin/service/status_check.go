package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	youtubeapi "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/api"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

type StatusCheckResult struct {
	Checked      int                    `json:"checked"`
	Online       int                    `json:"online"`
	Offline      int                    `json:"offline"`
	Updated      int                    `json:"updated"`
	Failed       int                    `json:"failed"`
	FailureList  []StreamerCheckFailure `json:"failure_list,omitempty"`
}

// StreamerCheckFailure captures details about a failed status check.
type StreamerCheckFailure struct {
	StreamerID   string `json:"streamer_id"`
	StreamerName string `json:"streamer_name"`
	ChannelID    string `json:"channel_id"`
	Error        string `json:"error"`
}

// StatusChecker inspects the stored roster and refreshes live status for each channel.
type StatusChecker struct {
	Streamers *streamers.Store
	Search    liveSearcher
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

	// Use sync primitives for concurrent checking
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		res  StatusCheckResult
	)

	// Check each streamer concurrently
	for _, record := range records {
		yt := record.Platforms.YouTube
		if yt == nil {
			continue
		}

		wg.Add(1)
		go func(rec streamers.Record, ytPlatform streamers.YouTubePlatform) {
			defer wg.Done()

			mu.Lock()
			res.Checked++
			mu.Unlock()

			// Get channel ID for error reporting
			channelID := strings.TrimSpace(ytPlatform.ChannelID)
			if channelID == "" {
				channelID = extractChannelID(ytPlatform.Topic)
			}

			outcome, err := c.checkYouTube(ctx, rec, ytPlatform, search)
			if err != nil {
				mu.Lock()
				res.Failed++

				// Log the failure with details
				streamerName := rec.Streamer.Alias
				if streamerName == "" {
					streamerName = rec.Streamer.ID
				}

				failure := StreamerCheckFailure{
					StreamerID:   rec.Streamer.ID,
					StreamerName: streamerName,
					ChannelID:    channelID,
					Error:        err.Error(),
				}
				res.FailureList = append(res.FailureList, failure)
				mu.Unlock()

				// Log to stdout for server logs
				if strings.Contains(err.Error(), "API") || strings.Contains(err.Error(), "quota") {
					// API errors are more important
					fmt.Printf("ERROR: Status check failed for streamer %q (channel %s): %v\n", streamerName, channelID, err)
				} else {
					fmt.Printf("WARN: Status check failed for streamer %q (channel %s): %v\n", streamerName, channelID, err)
				}

				return
			}

			mu.Lock()
			if outcome.live {
				res.Online++
			} else {
				res.Offline++
			}
			if outcome.updated {
				res.Updated++
			}
			mu.Unlock()
		}(record, *yt)
	}

	// Wait for all checks to complete
	wg.Wait()
	result = res

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
