package server

import (
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
		// Get current site's configuration
		siteConfig, siteErr := config.ResolveSite(s.siteKey, cfg)
		if siteErr != nil {
			data.Error = "Failed to resolve site configuration: " + siteErr.Error()
		} else {
			// YouTube configuration (site-specific) - defaults to enabled if not set
			youtubeEnabled := true
			if siteConfig.YouTube.Enabled != nil {
				youtubeEnabled = *siteConfig.YouTube.Enabled
			}
			data.YouTubeConfig = YouTubeConfigDisplay{
				Enabled:      youtubeEnabled,
				HubURL:       siteConfig.YouTube.HubURL,
				CallbackURL:  siteConfig.YouTube.CallbackURL,
				APIKey:       maskAPIKey(siteConfig.YouTube.APIKey),
				LeaseSeconds: siteConfig.YouTube.LeaseSeconds,
				Mode:         siteConfig.YouTube.Mode,
				Verify:       siteConfig.YouTube.Verify,
			}

			// Twitch configuration (site-specific) - defaults to enabled if not set
			twitchEnabled := true
			if siteConfig.Twitch.Enabled != nil {
				twitchEnabled = *siteConfig.Twitch.Enabled
			}
			data.TwitchConfig = TwitchConfigDisplay{
				Enabled:        twitchEnabled,
				CallbackURL:    siteConfig.Twitch.CallbackURL,
				ClientID:       maskAPIKey(siteConfig.Twitch.ClientID),
				ClientSecret:   maskSecret(siteConfig.Twitch.ClientSecret),
				EventSubSecret: maskSecret(siteConfig.Twitch.EventSubSecret),
			}

			// Facebook configuration (placeholder for now)
			data.FacebookConfig = PlatformConfigDisplay{
				Enabled: false,
				HubURL:  "Not configured",
			}
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
