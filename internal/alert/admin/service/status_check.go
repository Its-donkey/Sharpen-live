package service

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	Streamers  *streamers.Store
	Player     youtubePlayer
	FeedClient *http.Client
}

type youtubePlayer interface {
	LiveStatus(ctx context.Context, videoID string) (youtubeapi.LiveStatus, error)
}

const (
	defaultStatusCheckTimeout = 8 * time.Second
	maxFeedBytes              = 1 << 20
)

// CheckAll refreshes live status for every stored channel (currently YouTube only).
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

	player := c.player()
	client := c.httpClient()

	for _, record := range records {
		yt := record.Platforms.YouTube
		if yt == nil {
			continue
		}
		result.Checked++
		outcome, err := c.checkYouTube(ctx, record, *yt, player, client)
		if err != nil {
			result.Failed++
			continue
		}
		if outcome.live {
			result.Online++
		} else {
			result.Offline++
		}
		if outcome.updated {
			result.Updated++
		}
	}

	return result, nil
}

type checkOutcome struct {
	live    bool
	updated bool
}

func (c StatusChecker) checkYouTube(ctx context.Context, record streamers.Record, yt streamers.YouTubePlatform, player youtubePlayer, client *http.Client) (checkOutcome, error) {
	channelID := strings.TrimSpace(yt.ChannelID)
	if channelID == "" {
		channelID = extractChannelID(yt.Topic)
	}
	if channelID == "" {
		return checkOutcome{}, errors.New("youtube channel id missing")
	}

	candidates := statusVideoIDs(record)
	feedID, feedErr := c.fetchLatestVideoID(ctx, yt, client)
	if feedID != "" && !containsVideoID(candidates, feedID) {
		candidates = append(candidates, feedID)
	}
	if len(candidates) == 0 {
		if feedErr != nil {
			return checkOutcome{}, feedErr
		}
		return checkOutcome{}, errors.New("no video ids to check")
	}

	currentLive := record.Status != nil && record.Status.Live
	currentVideo := ""
	if record.Status != nil && record.Status.YouTube != nil {
		currentVideo = strings.TrimSpace(record.Status.YouTube.VideoID)
	}

	var lastErr error
	var hadResponse bool

	for _, videoID := range candidates {
		statusCtx, cancel := withTimeout(ctx, defaultStatusCheckTimeout)
		liveStatus, err := player.LiveStatus(statusCtx, videoID)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		hadResponse = true
		if liveStatus.IsOnline() && strings.EqualFold(liveStatus.ChannelID, channelID) {
			if currentLive && strings.EqualFold(currentVideo, videoID) {
				return checkOutcome{live: true, updated: false}, nil
			}
			_, err := c.Streamers.UpdateYouTubeLiveStatus(channelID, streamers.YouTubeLiveStatus{
				Live:      true,
				VideoID:   videoID,
				StartedAt: liveStatus.StartedAt,
			})
			if err != nil {
				return checkOutcome{}, err
			}
			return checkOutcome{live: true, updated: true}, nil
		}
	}

	if !hadResponse && lastErr != nil {
		return checkOutcome{}, lastErr
	}

	if currentLive {
		if _, err := c.Streamers.UpdateYouTubeLiveStatus(channelID, streamers.YouTubeLiveStatus{Live: false}); err != nil {
			return checkOutcome{}, err
		}
		return checkOutcome{live: false, updated: true}, nil
	}

	return checkOutcome{live: false, updated: false}, nil
}

func (c StatusChecker) fetchLatestVideoID(ctx context.Context, yt streamers.YouTubePlatform, client *http.Client) (string, error) {
	topic := strings.TrimSpace(yt.Topic)
	if topic == "" {
		channelID := strings.TrimSpace(yt.ChannelID)
		if channelID == "" {
			return "", errors.New("youtube topic missing")
		}
		topic = "https://www.youtube.com/feeds/videos.xml?channel_id=" + url.QueryEscape(channelID)
	}

	reqCtx, cancel := withTimeout(ctx, defaultStatusCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, topic, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch feed %s: %s", topic, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes))
	if err != nil {
		return "", err
	}

	var feed youtubeFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return "", err
	}
	for _, entry := range feed.Entries {
		videoID := strings.TrimSpace(entry.VideoID)
		if videoID != "" {
			return videoID, nil
		}
	}
	return "", nil
}

func (c StatusChecker) player() youtubePlayer {
	if c.Player != nil {
		return c.Player
	}
	return youtubeapi.NewPlayerClient(youtubeapi.PlayerClientOptions{})
}

func (c StatusChecker) httpClient() *http.Client {
	if c.FeedClient != nil {
		return c.FeedClient
	}
	return &http.Client{Timeout: defaultStatusCheckTimeout}
}

func statusVideoIDs(record streamers.Record) []string {
	if record.Status == nil || record.Status.YouTube == nil {
		return nil
	}
	id := strings.TrimSpace(record.Status.YouTube.VideoID)
	if id == "" {
		return nil
	}
	return []string{id}
}

func containsVideoID(list []string, target string) bool {
	for _, item := range list {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
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

type youtubeFeed struct {
	Entries []struct {
		VideoID string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
	} `xml:"entry"`
}
