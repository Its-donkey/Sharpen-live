package forms

import (
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"net/url"
	"strings"
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

func FirstPlatformURL(rows []model.PlatformFormRow) string {
	for _, r := range rows {
		if s := strings.TrimSpace(r.ChannelURL); s != "" {
			return s
		}
	}
	return ""
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
