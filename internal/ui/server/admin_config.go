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
	YouTubeConfig  PlatformConfigDisplay
	TwitchConfig   PlatformConfigDisplay
	FacebookConfig PlatformConfigDisplay
	YouTubeSites   []YouTubeSiteConfig
}

type PlatformConfigDisplay struct {
	Enabled     bool
	HubURL      string
	CallbackURL string
	APIKey      string
	LeaseSeconds int
	Mode        string
	Verify      string
	// Twitch/Facebook specific fields can be added later
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

	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.renderConfigPage(w, data)
		return
	}

	data.LoggedIn = true

	// Load configuration
	cfg, err := config.Load(s.configPath)
	if err != nil {
		data.Error = "Failed to load configuration: " + err.Error()
	} else {
		// YouTube configuration
		globalEnabled := true
		if cfg.YouTube.Enabled != nil {
			globalEnabled = *cfg.YouTube.Enabled
		}
		data.YouTubeConfig = PlatformConfigDisplay{
			Enabled:      globalEnabled,
			HubURL:       cfg.YouTube.HubURL,
			CallbackURL:  cfg.YouTube.CallbackURL,
			APIKey:       maskAPIKey(cfg.YouTube.APIKey),
			LeaseSeconds: cfg.YouTube.LeaseSeconds,
			Mode:         cfg.YouTube.Mode,
			Verify:       cfg.YouTube.Verify,
		}

		// Twitch configuration (placeholder for now)
		data.TwitchConfig = PlatformConfigDisplay{
			Enabled: false,
			HubURL:  "Not configured",
		}

		// Facebook configuration (placeholder for now)
		data.FacebookConfig = PlatformConfigDisplay{
			Enabled: false,
			HubURL:  "Not configured",
		}

		// Load YouTube site configurations
		youtubeConfigs, err := s.getYouTubeSiteConfigs()
		if err != nil {
			s.logger.Warn("admin_config", "failed to load YouTube site configs", map[string]any{
				"error": err.Error(),
			})
		} else {
			data.YouTubeSites = youtubeConfigs
		}
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
