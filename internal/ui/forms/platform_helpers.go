package forms

import (
	"net/url"
	"strings"

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
