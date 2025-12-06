package server

import (
	"fmt"
	"net/http"

	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
)

type configPageData struct {
	basePageData
	LoggedIn       bool
	Flash          string
	Error          string
	YouTubeConfig  YouTubeConfigDisplay
	TwitchConfig   TwitchConfigDisplay
	FacebookConfig PlatformConfigDisplay
	YouTubeSites   []YouTubeSiteConfig
}

// YouTubeConfigDisplay holds YouTube configuration for admin display.
type YouTubeConfigDisplay struct {
	Enabled      bool
	HubURL       string
	CallbackURL  string
	APIKey       string
	LeaseSeconds int
	Mode         string
	Verify       string
}

// TwitchConfigDisplay holds Twitch configuration for admin display.
type TwitchConfigDisplay struct {
	Enabled        bool
	CallbackURL    string
	ClientID       string
	ClientSecret   string
	EventSubSecret string
}

// PlatformConfigDisplay is a generic display struct for other platforms.
type PlatformConfigDisplay struct {
	Enabled     bool
	HubURL      string
	CallbackURL string
}

func (s *server) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	msg := r.URL.Query().Get("msg")
	errMsg := r.URL.Query().Get("err")

	base := s.buildBasePageData(r, "Configuration · Admin", "Platform integration configuration settings", "/admin/config")
	base.SecondaryAction = &navAction{
		Label: "Back to admin",
		Href:  "/admin",
	}
	base.Robots = "noindex, nofollow"

	data := configPageData{
		basePageData: base,
		Flash:        msg,
		Error:        errMsg,
	}

	// Load configuration first to populate platform status (shown even when not logged in)
	cfg, err := config.Load(s.configPath)
	if err != nil {
		data.Error = "Failed to load configuration: " + err.Error()
	} else {
		// Use GLOBAL platform config for display (cfg.Platforms)
		// Global YouTube configuration - defaults to enabled if not set
		youtubeEnabled := true
		if cfg.Platforms.YouTube.Enabled != nil {
			youtubeEnabled = *cfg.Platforms.YouTube.Enabled
		}
		data.YouTubeConfig = YouTubeConfigDisplay{
			Enabled:      youtubeEnabled,
			HubURL:       cfg.Platforms.YouTube.HubURL,
			CallbackURL:  cfg.Platforms.YouTube.CallbackURL,
			APIKey:       maskAPIKey(cfg.Platforms.YouTube.APIKey),
			LeaseSeconds: cfg.Platforms.YouTube.LeaseSeconds,
			Mode:         cfg.Platforms.YouTube.Mode,
			Verify:       cfg.Platforms.YouTube.Verify,
		}

		// Global Twitch configuration - defaults to enabled if not set
		twitchEnabled := true
		if cfg.Platforms.Twitch.Enabled != nil {
			twitchEnabled = *cfg.Platforms.Twitch.Enabled
		}
		data.TwitchConfig = TwitchConfigDisplay{
			Enabled:        twitchEnabled,
			CallbackURL:    cfg.Platforms.Twitch.CallbackURL,
			ClientID:       maskAPIKey(cfg.Platforms.Twitch.ClientID),
			ClientSecret:   maskSecret(cfg.Platforms.Twitch.ClientSecret),
			EventSubSecret: maskSecret(cfg.Platforms.Twitch.EventSubSecret),
		}

		// Facebook configuration (placeholder for now)
		data.FacebookConfig = PlatformConfigDisplay{
			Enabled: false,
			HubURL:  "Not configured",
		}
	}

	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.renderConfigPage(w, data)
		return
	}

	data.LoggedIn = true

	// Load YouTube site configurations (only when logged in)
	youtubeConfigs, err := s.getYouTubeSiteConfigs()
	if err != nil {
		s.logger.Warn("admin_config", "failed to load YouTube site configs", map[string]any{
			"error": err.Error(),
		})
	} else {
		data.YouTubeSites = youtubeConfigs
	}

	s.renderConfigPage(w, data)
}

func (s *server) renderConfigPage(w http.ResponseWriter, data configPageData) {
	tmpl, ok := s.templates["config"]
	if !ok {
		http.Error(w, "config template missing", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "config", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// maskAPIKey masks an API key for display, showing only first/last few characters
func maskAPIKey(apiKey string) string {
	if apiKey == "" {
		return "Not set"
	}
	if len(apiKey) <= 8 {
		return "••••••••"
	}
	return apiKey[:4] + "••••••••" + apiKey[len(apiKey)-4:]
}

// maskSecret masks a secret for display, showing only that it's configured or not
func maskSecret(secret string) string {
	if secret == "" {
		return "Not set"
	}
	return "••••••••••••"
}

// handleAdminPlatformSettings handles global platform enable/disable toggles
func (s *server) handleAdminPlatformSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/config?err=Invalid request", http.StatusSeeOther)
		return
	}

	token := s.adminTokenFromRequest(r)
	if token == "" {
		http.Redirect(w, r, "/admin/config?err=Log in to modify platform settings", http.StatusSeeOther)
		return
	}

	if !s.adminManager.Validate(token) {
		http.Redirect(w, r, "/admin/config?err=Invalid session. Please log in again.", http.StatusSeeOther)
		return
	}

	platform := r.FormValue("platform")
	enabled := r.FormValue("enabled") == "true"
	isGlobal := r.FormValue("global") == "true"

	if !isGlobal {
		http.Redirect(w, r, "/admin/config?err=Only global platform settings are supported", http.StatusSeeOther)
		return
	}

	cfg, err := config.Load(s.configPath)
	if err != nil {
		http.Redirect(w, r, "/admin/config?err="+fmt.Sprintf("Failed to load config: %v", err), http.StatusSeeOther)
		return
	}

	switch platform {
	case "youtube":
		cfg.Platforms.YouTube.Enabled = &enabled
	case "twitch":
		cfg.Platforms.Twitch.Enabled = &enabled
	default:
		http.Redirect(w, r, "/admin/config?err="+fmt.Sprintf("Unknown platform: %s", platform), http.StatusSeeOther)
		return
	}

	if err := config.Save(cfg, s.configPath); err != nil {
		s.logger.Warn("admin", "failed to save config after platform update", map[string]any{
			"platform": platform,
			"error":    err.Error(),
		})
		http.Redirect(w, r, "/admin/config?err="+fmt.Sprintf("Failed to save config: %v", err), http.StatusSeeOther)
		return
	}

	s.logger.Info("admin", "Global platform settings updated", map[string]any{
		"platform": platform,
		"enabled":  enabled,
	})

	statusMsg := fmt.Sprintf("%s globally %s", platform, map[bool]string{true: "enabled", false: "disabled"}[enabled])
	http.Redirect(w, r, "/admin/config?msg="+statusMsg, http.StatusSeeOther)
}
