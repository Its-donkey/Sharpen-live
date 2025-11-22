package streamers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

// FetchStreamers retrieves roster data from the API, falling back to bundled JSON if needed.
func FetchStreamers(ctx context.Context) ([]model.Streamer, error) {
	return fetchStreamersFromEndpoints(ctx, []string{
		"/api/streamers",
		"/streamers.json",
	})
}

// FetchStreamersFrom retrieves roster data from the specified API base URL.
func FetchStreamersFrom(ctx context.Context, apiBase string) ([]model.Streamer, error) {
	base := strings.TrimSpace(apiBase)
	if base == "" {
		return fetchStreamersFromEndpoints(ctx, []string{
			"/api/streamers",
			"/streamers.json",
		})
	}
	base = strings.TrimSuffix(base, "/")
	return fetchStreamersFromEndpoints(ctx, []string{
		base + "/api/streamers",
		base + "/streamers.json",
	})
}

func fetchStreamersFromEndpoints(ctx context.Context, endpoints []string) ([]model.Streamer, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	var attemptErr error

	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			attemptErr = err
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			attemptErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			if err != nil {
				attemptErr = err
			} else {
				attemptErr = fmt.Errorf("fetch %s failed: %s", endpoint, resp.Status)
			}
			continue
		}

		if data, err := decodeStreamers(body); err == nil {
			return data, nil
		} else {
			attemptErr = err
		}
	}

	if attemptErr == nil {
		attemptErr = errors.New("no streamer endpoint responded successfully")
	}
	return fallbackStreamers(), attemptErr
}

func decodeStreamers(payload []byte) ([]model.Streamer, error) {
	var direct []model.Streamer
	if err := json.Unmarshal(payload, &direct); err == nil && direct != nil {
		return direct, nil
	}

	var serverResp model.ServerStreamersResponse
	if err := json.Unmarshal(payload, &serverResp); err == nil && serverResp.Streamers != nil {
		return mapServerStreamers(serverResp.Streamers), nil
	}

	var wrapped model.WrappedStreamers
	if err := json.Unmarshal(payload, &wrapped); err == nil && wrapped.Streamers != nil {
		return wrapped.Streamers, nil
	}

	return nil, fmt.Errorf("unexpected response shape")
}

func mapServerStreamers(records []model.ServerStreamerRecord) []model.Streamer {
	online := make([]model.Streamer, 0, len(records))
	offline := make([]model.Streamer, 0, len(records))
	for _, rec := range records {
		name := strings.TrimSpace(rec.Streamer.Alias)
		if name == "" {
			name = strings.TrimSpace(rec.Streamer.ID)
		}

		state, label := deriveStatus(rec.Status)

		mapped := model.Streamer{
			ID:          rec.Streamer.ID,
			Name:        name,
			Description: strings.TrimSpace(rec.Streamer.Description),
			Status:      state,
			StatusLabel: label,
			Languages:   append([]string(nil), rec.Streamer.Languages...),
			Platforms:   collectPlatforms(rec.Platforms, rec.Status),
		}
		if rec.Status.Live {
			online = append(online, mapped)
		} else {
			offline = append(offline, mapped)
		}
	}
	return append(online, offline...)
}

func deriveStatus(status model.ServerStatus) (string, string) {
	if status.Live {
		return "online", "Online"
	}
	if len(status.Platforms) > 0 {
		return "busy", "Workshop"
	}
	return "offline", "Offline"
}

func collectPlatforms(details model.ServerPlatformDetails, status model.ServerStatus) []model.Platform {
	var platforms []model.Platform
	if yt := details.YouTube; yt != nil {
		if url := youtubeLiveURL(yt, status.YouTube); url != "" {
			platforms = append(platforms, model.Platform{
				Name:       "YouTube",
				ChannelURL: url,
			})
		}
	}
	if tw := details.Twitch; tw != nil {
		platforms = append(platforms, model.Platform{
			Name:       "Twitch",
			ChannelURL: twitchChannelURL(tw),
		})
	}
	if fb := details.Facebook; fb != nil {
		platforms = append(platforms, model.Platform{
			Name:       "Facebook",
			ChannelURL: facebookPageURL(fb),
		})
	}
	return platforms
}

func youtubeChannelURL(details *model.ServerYouTubePlatform) string {
	handle := strings.TrimSpace(details.Handle)
	if handle != "" {
		if !strings.HasPrefix(handle, "@") {
			handle = "@" + handle
		}
		return "https://www.youtube.com/" + handle
	}
	channel := strings.TrimSpace(details.ChannelID)
	if channel != "" {
		return "https://www.youtube.com/channel/" + channel
	}
	const feedPrefix = "https://www.youtube.com/xml/feeds/videos.xml?channel_id="
	if topic := strings.TrimSpace(details.Topic); strings.HasPrefix(topic, feedPrefix) {
		return "https://www.youtube.com/channel/" + topic[len(feedPrefix):]
	}
	return ""
}

func youtubeLiveURL(details *model.ServerYouTubePlatform, status *model.ServerYouTubeStatus) string {
	if status == nil || !status.Live {
		return ""
	}
	if videoID := strings.TrimSpace(status.VideoID); videoID != "" {
		return "https://www.youtube.com/watch?v=" + videoID
	}
	return youtubeChannelURL(details)
}

func twitchChannelURL(details *model.ServerTwitchPlatform) string {
	username := strings.TrimSpace(details.Username)
	if username == "" {
		return ""
	}
	return "https://www.twitch.tv/" + username
}

func facebookPageURL(details *model.ServerFacebookPlatform) string {
	pageID := strings.TrimSpace(details.PageID)
	if pageID == "" {
		return ""
	}
	return "https://www.facebook.com/" + pageID
}

func fallbackStreamers() []model.Streamer {
	return []model.Streamer{
		{
			ID:          "edgecrafter",
			Name:        "EdgeCrafter",
			Description: "Specializes in chef knives · Est. wait time 10 min",
			Status:      "online",
			StatusLabel: "Online",
			Languages:   []string{"English"},
			Platforms: []model.Platform{
				{Name: "Twitch", ChannelURL: "https://www.twitch.tv/edgecrafter"},
				{Name: "YouTube", ChannelURL: "https://www.youtube.com/@edgecrafter"},
			},
		},
		{
			ID:          "zen-edge",
			Name:        "Zen Edge Studio",
			Description: "Waterstone specialist · Accepting rush orders",
			Status:      "busy",
			StatusLabel: "Workshop",
			Languages:   []string{"English", "Japanese"},
			Platforms: []model.Platform{
				{Name: "YouTube", ChannelURL: "https://www.youtube.com/@zenedgestudio"},
			},
		},
		{
			ID:          "forge-feather",
			Name:        "Forge & Feather",
			Description: "Damascus patterns · Next stream 19:00 UTC",
			Status:      "offline",
			StatusLabel: "Offline",
			Languages:   []string{"French"},
			Platforms: []model.Platform{
				{Name: "Kick", ChannelURL: "https://kick.com/forgeandfeather"},
				{Name: "Twitch", ChannelURL: "https://www.twitch.tv/forgeandfeather"},
			},
		},
		{
			ID:          "honbazuke",
			Name:        "Honbazuke Pro",
			Description: "Premium partners · Bookings open",
			Status:      "online",
			StatusLabel: "Online",
			Languages:   []string{"English", "German"},
			Platforms: []model.Platform{
				{Name: "Twitch", ChannelURL: "https://www.twitch.tv/honbazukepro"},
				{Name: "Instagram Live", ChannelURL: "https://www.instagram.com/honbazukepro/"},
			},
		},
		{
			ID:          "sharp-true",
			Name:        "Sharp & True",
			Description: "Mobile serviceeeee · On-site events available",
			Status:      "offline",
			StatusLabel: "Offline",
			Languages:   []string{"Spanish"},
			Platforms: []model.Platform{
				{Name: "YouTube", ChannelURL: "https://www.youtube.com/@sharpandtrue"},
				{Name: "Facebook Live", ChannelURL: "https://www.facebook.com/sharpandtrue/live"},
			},
		},
	}
}
