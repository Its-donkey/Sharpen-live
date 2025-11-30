package server

import (
	"os"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
)

func applyDefaults(opts Options, site config.SiteConfig) Options {
	fallbackApp := config.CatchAllAppConfig()
	if opts.Listen == "" {
		addr := strings.TrimSpace(site.Server.Addr)
		port := strings.TrimSpace(site.Server.Port)
		if addr != "" || port != "" {
			if port != "" && !strings.HasPrefix(port, ":") {
				opts.Listen = addr + ":" + port
			} else {
				opts.Listen = addr + port
			}
		}
		if opts.Listen == "" {
			opts.Listen = "127.0.0.1:4173"
		}
	}
	if opts.TemplatesDir == "" {
		if site.App.Templates != "" {
			opts.TemplatesDir = site.App.Templates
		} else {
			opts.TemplatesDir = fallbackApp.Templates
		}
	}
	if opts.AssetsDir == "" {
		if site.App.Assets != "" {
			opts.AssetsDir = site.App.Assets
		} else {
			opts.AssetsDir = fallbackApp.Assets
		}
	}
	if opts.LogDir == "" {
		if site.App.Logs != "" {
			opts.LogDir = site.App.Logs
		} else {
			opts.LogDir = fallbackApp.Logs
		}
	}
	if opts.DataDir == "" {
		if site.App.Data != "" {
			opts.DataDir = site.App.Data
		} else {
			opts.DataDir = fallbackApp.Data
		}
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = "config.json"
	}
	return opts
}

func switchToCatchAll(cfg config.Config, opts Options) (config.SiteConfig, Options) {
	fallback := config.CatchAllSite(cfg)
	opts.Site = fallback.Key
	opts.TemplatesDir = ""
	opts.AssetsDir = ""
	opts.LogDir = ""
	opts.DataDir = ""
	opts = applyDefaults(opts, fallback)
	return fallback, opts
}

func fileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func statusClass(status string) string {
	state := strings.ToLower(strings.TrimSpace(status))
	switch state {
	case "online", "busy", "offline":
		return state
	case "approved":
		return "status-approved"
	case "rejected":
		return "status-rejected"
	default:
		return "status-pending"
	}
}

func statusLabel(status string) string {
	if label, ok := model.StatusLabels[status]; ok {
		return label
	}
	return status
}

func twitchChannelURL(handle string) string {
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return ""
	}
	return "https://www.twitch.tv/" + handle
}

func facebookPageURL(handle string) string {
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return ""
	}
	return "https://www.facebook.com/" + handle
}
