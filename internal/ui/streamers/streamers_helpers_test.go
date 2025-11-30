package streamers

import (
	"testing"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	youtubeui "github.com/Its-donkey/Sharpen-live/internal/ui/platforms/youtube"
)

func TestDeriveStatus(t *testing.T) {
	tests := []struct {
		name  string
		input model.ServerStatus
		state string
		label string
	}{
		{name: "live", input: model.ServerStatus{Live: true}, state: "online", label: "Online"},
		{name: "busy", input: model.ServerStatus{Platforms: []string{"YouTube"}}, state: "busy", label: "Workshop"},
		{name: "offline", input: model.ServerStatus{}, state: "offline", label: "Offline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, label := deriveStatus(tt.input)
			if state != tt.state || label != tt.label {
				t.Fatalf("expected %s/%s got %s/%s", tt.state, tt.label, state, label)
			}
		})
	}
}

func TestCollectPlatforms(t *testing.T) {
	status := model.ServerStatus{
		YouTube: &model.ServerYouTubeStatus{Live: true, VideoID: "123"},
	}
	details := model.ServerPlatformDetails{
		YouTube: &model.ServerYouTubePlatform{Handle: "@edge"},
		Twitch:  &model.ServerTwitchPlatform{Username: "forge"},
		Facebook: &model.ServerFacebookPlatform{
			PageID: "page-1",
		},
	}
	platforms := collectPlatforms(details, status)
	if len(platforms) != 3 {
		t.Fatalf("expected 3 platforms got %d", len(platforms))
	}
	if platforms[0].Name != "YouTube" || platforms[0].ChannelURL == "" {
		t.Fatalf("expected YouTube entry with url: %#v", platforms[0])
	}
	if platforms[1].Name != "Twitch" || platforms[1].ChannelURL == "" {
		t.Fatalf("expected Twitch entry with url: %#v", platforms[1])
	}
	if platforms[2].Name != "Facebook" || platforms[2].ChannelURL == "" {
		t.Fatalf("expected Facebook entry with url: %#v", platforms[2])
	}
}

func TestYoutubeChannelURL(t *testing.T) {
	tests := []struct {
		name string
		det  *model.ServerYouTubePlatform
		want string
	}{
		{name: "handle with at", det: &model.ServerYouTubePlatform{Handle: "@edge"}, want: "https://www.youtube.com/@edge"},
		{name: "handle w/out at", det: &model.ServerYouTubePlatform{Handle: "edge"}, want: "https://www.youtube.com/@edge"},
		{name: "channel id", det: &model.ServerYouTubePlatform{ChannelID: "abc"}, want: "https://www.youtube.com/channel/abc"},
		{name: "topic feed", det: &model.ServerYouTubePlatform{Topic: "https://www.youtube.com/xml/feeds/videos.xml?channel_id=xyz"}, want: "https://www.youtube.com/channel/xyz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := youtubeui.ChannelURLFromDetails(tt.det); got != tt.want {
				t.Fatalf("expected %q got %q", tt.want, got)
			}
		})
	}
}
