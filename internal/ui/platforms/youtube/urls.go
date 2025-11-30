package youtube

import (
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

// ChannelURLFromPlatform builds a canonical channel URL from a streamer record's YouTube platform details.
func ChannelURLFromPlatform(yt *streamers.YouTubePlatform) string {
	if yt == nil {
		return ""
	}
	if url := ChannelURLFromHandle(yt.Handle); url != "" {
		return url
	}
	if id := strings.TrimSpace(yt.ChannelID); id != "" {
		return "https://www.youtube.com/channel/" + id
	}
	return ""
}

// ChannelURLFromHandle returns the canonical channel URL for a YouTube handle.
func ChannelURLFromHandle(handle string) string {
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return ""
	}
	if !strings.HasPrefix(handle, "@") {
		handle = "@" + handle
	}
	return "https://www.youtube.com/" + handle
}

// ChannelURLFromDetails builds a channel URL from server response details.
func ChannelURLFromDetails(details *model.ServerYouTubePlatform) string {
	if details == nil {
		return ""
	}
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

// LiveURLFromDetails returns either a live video link or a channel URL depending on status.
func LiveURLFromDetails(details *model.ServerYouTubePlatform, status *model.ServerYouTubeStatus) string {
	if status == nil || !status.Live {
		return ""
	}
	if videoID := strings.TrimSpace(status.VideoID); videoID != "" {
		return "https://www.youtube.com/watch?v=" + videoID
	}
	return ChannelURLFromDetails(details)
}

// LiveURLFromChannel appends the live path to a channel URL.
func LiveURLFromChannel(channelURL string) string {
	channelURL = strings.TrimSpace(channelURL)
	if channelURL == "" {
		return ""
	}
	return channelURL + "/live"
}
