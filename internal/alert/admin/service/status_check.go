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

	twitchapi "github.com/Its-donkey/Sharpen-live/internal/alert/platforms/twitch/api"
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
	Streamers          *streamers.Store
	Search             liveSearcher
	TwitchClientID     string
	TwitchClientSecret string
}

type liveSearcher interface {
	LiveNow(ctx context.Context, channelID string) (youtubeapi.SearchLiveResult, error)
}

const defaultStatusCheckTimeout = 15 * time.Second

// CheckAll refreshes live status for every stored channel (YouTube and Twitch).
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

	// Check YouTube streamers concurrently
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

	// Check Twitch streamers (batch API call is more efficient)
	if c.TwitchClientID != "" && c.TwitchClientSecret != "" {
		twitchResults, twitchErr := c.checkAllTwitch(ctx, records)
		if twitchErr != nil {
			fmt.Printf("ERROR: Twitch batch status check failed: %v\n", twitchErr)
		} else {
			mu.Lock()
			res.Checked += twitchResults.Checked
			res.Online += twitchResults.Online
			res.Offline += twitchResults.Offline
			res.Updated += twitchResults.Updated
			res.Failed += twitchResults.Failed
			res.FailureList = append(res.FailureList, twitchResults.FailureList...)
			mu.Unlock()
		}
	}

	// Wait for all YouTube checks to complete
	wg.Wait()
	result = res

	return result, nil
}

// checkAllTwitch checks live status for all Twitch streamers in a single batch API call.
func (c StatusChecker) checkAllTwitch(ctx context.Context, records []streamers.Record) (StatusCheckResult, error) {
	var result StatusCheckResult

	// Collect all broadcaster IDs
	var broadcasterIDs []string
	broadcasterToRecord := make(map[string]streamers.Record)
	for _, record := range records {
		if record.Platforms.Twitch != nil && record.Platforms.Twitch.BroadcasterID != "" {
			broadcasterIDs = append(broadcasterIDs, record.Platforms.Twitch.BroadcasterID)
			broadcasterToRecord[record.Platforms.Twitch.BroadcasterID] = record
		}
	}

	if len(broadcasterIDs) == 0 {
		return result, nil
	}

	result.Checked = len(broadcasterIDs)

	// Create Twitch API client
	httpClient := &http.Client{Timeout: 30 * time.Second}
	auth := twitchapi.NewAuthenticator(httpClient, c.TwitchClientID, c.TwitchClientSecret)

	// Batch check all streamers
	statusCtx, cancel := withTimeout(ctx, 35*time.Second)
	defer cancel()

	streams, err := twitchapi.GetStreams(statusCtx, httpClient, auth, broadcasterIDs)
	if err != nil {
		return result, fmt.Errorf("twitch streams API: %w", err)
	}

	// Update status for each streamer
	for broadcasterID, record := range broadcasterToRecord {
		streamerName := record.Streamer.Alias
		if streamerName == "" {
			streamerName = record.Streamer.ID
		}

		currentLive := record.Status != nil && record.Status.Twitch != nil && record.Status.Twitch.Live
		currentStreamID := ""
		if record.Status != nil && record.Status.Twitch != nil {
			currentStreamID = record.Status.Twitch.StreamID
		}

		if streamResult, isLive := streams[broadcasterID]; isLive && streamResult.IsLive {
			// Streamer is live
			if currentLive && currentStreamID == streamResult.StreamID {
				// No change
				result.Online++
				continue
			}

			_, err := c.Streamers.SetTwitchLive(broadcasterID, streamResult.StreamID, streamResult.StartedAt)
			if err != nil {
				result.Failed++
				result.FailureList = append(result.FailureList, StreamerCheckFailure{
					StreamerID:   record.Streamer.ID,
					StreamerName: streamerName,
					ChannelID:    broadcasterID,
					Error:        fmt.Sprintf("twitch set live: %v", err),
				})
				continue
			}
			result.Online++
			result.Updated++
		} else {
			// Streamer is offline
			if !currentLive {
				// No change
				result.Offline++
				continue
			}

			_, err := c.Streamers.ClearTwitchLive(broadcasterID)
			if err != nil {
				result.Failed++
				result.FailureList = append(result.FailureList, StreamerCheckFailure{
					StreamerID:   record.Streamer.ID,
					StreamerName: streamerName,
					ChannelID:    broadcasterID,
					Error:        fmt.Sprintf("twitch clear live: %v", err),
				})
				continue
			}
			result.Offline++
			result.Updated++
		}
	}

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
