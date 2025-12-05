package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	"github.com/Its-donkey/Sharpen-live/internal/alert/settings"
)

// YouTubeSiteConfig represents YouTube configuration for a site.
type YouTubeSiteConfig struct {
	SiteKey  string
	SiteName string
	Enabled  bool
}

// isYouTubeEnabled checks if YouTube integration is enabled for the current site.
func (s *server) isYouTubeEnabled() bool {
	dataDir := s.streamersStore.Path()
	if strings.HasSuffix(dataDir, "/streamers.json") {
		dataDir = strings.TrimSuffix(dataDir, "/streamers.json")
	}
	settingsStore := settings.New(dataDir)
	return settingsStore.IsYouTubeEnabled()
}

func (s *server) handleAdminYouTubeSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.redirectAdmin(w, r, "", "Invalid request.")
		return
	}

	token := s.adminTokenFromRequest(r)
	if token == "" {
		s.redirectAdmin(w, r, "", "Log in to modify YouTube settings.")
		return
	}

	if !s.adminManager.Validate(token) {
		s.redirectAdmin(w, r, "", "Invalid session. Please log in again.")
		return
	}

	siteKey := strings.TrimSpace(r.FormValue("site_key"))
	enabled := r.FormValue("youtube_enabled") == "true"

	// Determine which site to update
	targetSiteKey := siteKey
	if targetSiteKey == "" {
		targetSiteKey = s.siteKey
	}

	// Load the appropriate settings store
	var dataDir string
	if targetSiteKey == "" || targetSiteKey == config.DefaultSiteKey {
		dataDir = s.streamersStore.Path()
		if strings.HasSuffix(dataDir, "/streamers.json") {
			dataDir = strings.TrimSuffix(dataDir, "/streamers.json")
		}
	} else {
		// Find the site's data directory
		cfg, err := config.Load(s.configPath)
		if err != nil {
			s.redirectAdmin(w, r, "", fmt.Sprintf("Failed to load config: %v", err))
			return
		}
		site, err := config.ResolveSite(targetSiteKey, cfg)
		if err != nil {
			s.redirectAdmin(w, r, "", fmt.Sprintf("Site %q not found", targetSiteKey))
			return
		}
		dataDir = site.App.Data
	}

	settingsStore := settings.New(dataDir)
	if err := settingsStore.SetYouTubeEnabled(enabled); err != nil {
		s.logger.Warn("admin", "failed to update YouTube settings", map[string]any{
			"site":  targetSiteKey,
			"error": err.Error(),
		})
		s.redirectAdmin(w, r, "", fmt.Sprintf("Failed to update settings: %v", err))
		return
	}

	s.logger.Info("admin", "YouTube settings updated", map[string]any{
		"site":    targetSiteKey,
		"enabled": enabled,
	})

	statusMsg := "YouTube enabled"
	if !enabled {
		statusMsg = "YouTube disabled"
	}
	if targetSiteKey != "" && targetSiteKey != config.DefaultSiteKey {
		statusMsg = fmt.Sprintf("%s for %s", statusMsg, targetSiteKey)
	}

	s.redirectAdmin(w, r, statusMsg, "")
}

// getYouTubeSiteConfigs returns YouTube configuration for all sites or current site.
func (s *server) getYouTubeSiteConfigs() ([]YouTubeSiteConfig, error) {
	// If on default-site, show all sites
	if s.siteKey == config.DefaultSiteKey {
		cfg, err := config.Load(s.configPath)
		if err != nil {
			return nil, err
		}

		var configs []YouTubeSiteConfig

		// Add each configured site
		for key, site := range cfg.Sites {
			settingsStore := settings.New(site.App.Data)
			configs = append(configs, YouTubeSiteConfig{
				SiteKey:  key,
				SiteName: site.Name,
				Enabled:  settingsStore.IsYouTubeEnabled(),
			})
		}

		return configs, nil
	}

	// Otherwise, just return current site
	dataDir := s.streamersStore.Path()
	if strings.HasSuffix(dataDir, "/streamers.json") {
		dataDir = strings.TrimSuffix(dataDir, "/streamers.json")
	}

	settingsStore := settings.New(dataDir)
	return []YouTubeSiteConfig{
		{
			SiteKey:  s.siteKey,
			SiteName: s.siteName,
			Enabled:  settingsStore.IsYouTubeEnabled(),
		},
	}, nil
}
