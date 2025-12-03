package forms

import (
	"net/url"
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	youtubeui "github.com/Its-donkey/Sharpen-live/internal/ui/platforms/youtube"
)

func DerivePlatformLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "@") {
		return raw
	}
	if parsed, err := url.Parse(raw); err == nil {
		seg := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(seg) > 0 && strings.HasPrefix(seg[0], "@") {
			return seg[0]
		}
		if host := parsed.Hostname(); host != "" {
			return host
		}
	}
	return raw
}

func CanonicalizeChannelInput(raw string) string {
	return youtubeui.CanonicalizeChannelInput(raw)
}

func FirstPlatformURL(rows []model.PlatformFormRow) string {
	for _, r := range rows {
		if s := strings.TrimSpace(r.ChannelURL); s != "" {
			return s
		}
	}
	return ""
}

// DetectPlatformFromURL attempts to identify the platform from a URL
func DetectPlatformFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())

	// Check for known platforms
	switch {
	case strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be"):
		return "youtube"
	case strings.Contains(host, "twitch.tv"):
		return "twitch"
	case strings.Contains(host, "facebook.com") || strings.Contains(host, "fb.com"):
		return "facebook"
	case strings.Contains(host, "kick.com"):
		return "kick"
	case strings.Contains(host, "rumble.com"):
		return "rumble"
	case strings.Contains(host, "tiktok.com"):
		return "tiktok"
	default:
		// Return the hostname as the platform if we can't identify it
		if host != "" {
			return host
		}
		return "other"
	}
}

// BuildPlatformsMap converts platform form rows into a map of platforms
func BuildPlatformsMap(rows []model.PlatformFormRow) map[string]submissions.PlatformInfo {
	platforms := make(map[string]submissions.PlatformInfo)

	for _, row := range rows {
		urlStr := strings.TrimSpace(row.ChannelURL)
		if urlStr == "" {
			continue
		}

		// Detect platform from URL or use preset as a fallback
		detectedPlatform := strings.ToLower(DetectPlatformFromURL(urlStr))
		if detectedPlatform == "" {
			detectedPlatform = strings.ToLower(strings.TrimSpace(row.Preset))
		}
		if detectedPlatform == "" {
			detectedPlatform = "other"
		}

		// Only keep the first entry per platform
		if _, exists := platforms[detectedPlatform]; exists {
			continue
		}

		key := detectedPlatform

		platforms[key] = submissions.PlatformInfo{
			URL:       urlStr,
			Platform:  detectedPlatform,
			Preset:    strings.TrimSpace(row.Preset),
			Handle:    strings.TrimSpace(row.Handle),
			ChannelID: strings.TrimSpace(row.ChannelID),
			Label:     DerivePlatformLabel(urlStr),
		}
	}

	return platforms
}
