package server

import (
	"fmt"
	"net/http"

	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
)

// YouTubeSiteConfig represents YouTube configuration for a site.
type YouTubeSiteConfig struct {
	SiteKey  string
	SiteName string
	Enabled  bool
}

// isYouTubeEnabled checks if YouTube integration is enabled for the current site.
func (s *server) isYouTubeEnabled() bool {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		s.logger.Warn("admin", "failed to load config for YouTube check", map[string]any{
			"error": err.Error(),
		})
		return true // Default to enabled if config can't be loaded
	}

	site, err := config.ResolveSite(s.siteKey, cfg)
	if err != nil {
		s.logger.Warn("admin", "failed to resolve site for YouTube check", map[string]any{
			"siteKey": s.siteKey,
			"error":   err.Error(),
		})
		return true // Default to enabled if site can't be resolved
	}

	// If YouTubeEnabled is nil, use global default (true)
	if site.YouTubeEnabled == nil {
		return true
	}

	return *site.YouTubeEnabled
}

// isYouTubeEnabledForSiteKey checks if YouTube integration is enabled for a specific site key.
func isYouTubeEnabledForSiteKey(configPath, siteKey string) bool {
	cfg, err := config.Load(configPath)
	if err != nil {
		return true // Default to enabled if config can't be loaded
	}

	site, err := config.ResolveSite(siteKey, cfg)
	if err != nil {
		return true // Default to enabled if site can't be resolved
	}

	// If YouTubeEnabled is nil, use global default (true)
	if site.YouTubeEnabled == nil {
		return true
	}

	return *site.YouTubeEnabled
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

	siteKey := r.FormValue("site_key")
	enabled := r.FormValue("youtube_enabled") == "true"
	isGlobal := r.FormValue("global") == "true"

	// Load config
	cfg, err := config.Load(s.configPath)
	if err != nil {
		s.redirectAdmin(w, r, "", fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Handle global YouTube toggle
	if isGlobal {
		cfg.YouTube.Enabled = &enabled
		if err := config.Save(cfg, s.configPath); err != nil {
			s.logger.Warn("admin", "failed to save config after global YouTube update", map[string]any{
				"error": err.Error(),
			})
			http.Redirect(w, r, "/admin/config?err="+fmt.Sprintf("Failed to save config: %v", err), http.StatusSeeOther)
			return
		}

		s.logger.Info("admin", "Global YouTube settings updated", map[string]any{
			"enabled": enabled,
		})

		statusMsg := "YouTube globally enabled"
		if !enabled {
			statusMsg = "YouTube globally disabled"
		}
		http.Redirect(w, r, "/admin/config?msg="+statusMsg, http.StatusSeeOther)
		return
	}

	// Determine which site to update
	targetSiteKey := siteKey
	if targetSiteKey == "" {
		targetSiteKey = s.siteKey
	}

	// Update the YouTube enabled setting for the target site
	if targetSiteKey == "" || targetSiteKey == config.AlertserverKey {
		// For empty or default site key, this would be updating the base config
		// But we're using per-site settings, so log a warning
		s.logger.Warn("admin", "attempted to update YouTube for base config", map[string]any{
			"targetSiteKey": targetSiteKey,
		})
		http.Redirect(w, r, "/admin/config?err=Cannot update YouTube settings for base configuration. Please use a specific site.", http.StatusSeeOther)
		return
	}

	// Find and update the site in config
	site, exists := cfg.Sites[targetSiteKey]
	if !exists {
		http.Redirect(w, r, "/admin/config?err="+fmt.Sprintf("Site %q not found", targetSiteKey), http.StatusSeeOther)
		return
	}

	// Update the YouTubeEnabled field
	site.YouTubeEnabled = &enabled
	cfg.Sites[targetSiteKey] = site

	// Save config back to file
	if err := config.Save(cfg, s.configPath); err != nil {
		s.logger.Warn("admin", "failed to save config after YouTube update", map[string]any{
			"site":  targetSiteKey,
			"error": err.Error(),
		})
		http.Redirect(w, r, "/admin/config?err="+fmt.Sprintf("Failed to save config: %v", err), http.StatusSeeOther)
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
	statusMsg = fmt.Sprintf("%s for %s", statusMsg, targetSiteKey)

	http.Redirect(w, r, "/admin/config?msg="+statusMsg, http.StatusSeeOther)
}

// getYouTubeSiteConfigs returns YouTube configuration for all sites or current site.
func (s *server) getYouTubeSiteConfigs() ([]YouTubeSiteConfig, error) {
	cfg, err := config.Load(s.configPath)
	if err != nil {
		return nil, err
	}

	// If on default-site, show all sites
	if s.siteKey == config.AlertserverKey {
		var configs []YouTubeSiteConfig

		// Add each configured site
		for key, site := range cfg.Sites {
			enabled := true
			if site.YouTubeEnabled != nil {
				enabled = *site.YouTubeEnabled
			}
			configs = append(configs, YouTubeSiteConfig{
				SiteKey:  key,
				SiteName: site.Name,
				Enabled:  enabled,
			})
		}

		return configs, nil
	}

	// Otherwise, just return current site
	site, err := config.ResolveSite(s.siteKey, cfg)
	if err != nil {
		return nil, err
	}

	enabled := true
	if site.YouTubeEnabled != nil {
		enabled = *site.YouTubeEnabled
	}

	return []YouTubeSiteConfig{
		{
			SiteKey:  s.siteKey,
			SiteName: s.siteName,
			Enabled:  enabled,
		},
	}, nil
}
