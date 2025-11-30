package youtube

import (
	"net/url"
	"strings"
)

// CanonicalizeChannelInput normalizes free-form channel entries into a consistent URL shape.
func CanonicalizeChannelInput(raw string) string {
	t := strings.TrimSpace(raw)
	if t == "" {
		return ""
	}
	l := strings.ToLower(t)
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		return t
	}
	if h := extractHandle(t); h != "" {
		return buildURLFromHandle(h, "youtube")
	}
	if strings.HasPrefix(l, "youtube.com/") || strings.HasPrefix(l, "www.youtube.com/") || strings.HasPrefix(l, "m.youtube.com/") {
		return "https://" + t
	}
	if strings.HasPrefix(l, "youtu.be/") {
		return "https://" + t
	}
	return t
}

func extractHandle(raw string) string {
	t := strings.TrimSpace(raw)
	if strings.HasPrefix(t, "@") && len(t) > 1 {
		return t
	}
	return ""
}

func inferHandleFromURL(raw string) string {
	t := strings.TrimSpace(raw)
	if t == "" {
		return ""
	}
	if strings.HasPrefix(t, "@") {
		return t
	}
	if p, err := url.Parse(t); err == nil {
		seg := strings.Split(strings.Trim(p.Path, "/"), "/")
		for _, s := range seg {
			if strings.HasPrefix(s, "@") {
				return s
			}
		}
	}
	return ""
}

func resolvePlatformPreset(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "twitch":
		return "twitch"
	case "facebook":
		return "facebook"
	default:
		return "youtube"
	}
}

func buildURLFromHandle(handle, preset string) string {
	h := strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if h == "" {
		return ""
	}
	switch resolvePlatformPreset(preset) {
	case "twitch":
		return "https://www.twitch.tv/" + h
	case "facebook":
		return "https://www.facebook.com/" + h
	default:
		return "https://www.youtube.com/@" + h
	}
}
