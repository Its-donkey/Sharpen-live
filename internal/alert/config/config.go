// Package config loads and normalises alert-server configuration files.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	defaultAddr         = "127.0.0.1"
	defaultPort         = ":8880"
	defaultData         = "data/alertserver"
	defaultTemplatesDir = "ui/sites/default-site/templates"
	defaultAssetsDir    = "ui/sites/default-site"
	alertserverName     = "Alertserver Admin"
	AlertserverKey      = "alertserver"
)

// YouTubeConfig captures the WebSub-specific defaults persisted in config files.
type YouTubeConfig struct {
	Enabled         *bool  `json:"enabled,omitempty"`
	HubURL          string `json:"hub_url"`
	CallbackURL     string `json:"callback_url"`
	LocalWebSubPath string `json:"local_websub_path"` // Optional: local handler path if different from callback URL path
	LeaseSeconds    int    `json:"lease_seconds"`
	Mode            string `json:"mode"`
	Verify          string `json:"verify"`
	APIKey          string `json:"api_key"`
}

// TwitchConfig captures Twitch EventSub and API configuration.
type TwitchConfig struct {
	Enabled        *bool  `json:"enabled,omitempty"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
	EventSubSecret string `json:"eventsub_secret"` // Shared secret for verifying EventSub webhook signatures
	CallbackURL    string `json:"callback_url"`    // Public URL for EventSub callbacks
}

// ServerConfig configures the HTTP listener used by alert-server.
type ServerConfig struct {
	Addr string `json:"addr"`
	Port string `json:"port"`
}

// AppConfig configures server-rendered assets/templates and data locations.
type AppConfig struct {
	Templates string `json:"templates"`
	Assets    string `json:"assets"`
	Data      string `json:"data"`
	Name      string `json:"name"`
}

// SiteConfig captures per-site overrides for server/app/platform settings.
type SiteConfig struct {
	Key         string
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Server      ServerConfig  `json:"server"`
	App         AppConfig     `json:"app"`
	YouTube     YouTubeConfig `json:"youtube"`
	Twitch      TwitchConfig  `json:"twitch"`
}

// AdminConfig stores credentials for admin-authenticated APIs.
type AdminConfig struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	TokenTTLSeconds int    `json:"token_ttl_seconds"`
}

// PlatformsConfig holds global platform defaults shared across all sites.
type PlatformsConfig struct {
	YouTube YouTubeConfig `json:"youtube"`
	Twitch  TwitchConfig  `json:"twitch"`
}

// Config represents the combined runtime settings parsed from config.json.
type Config struct {
	Server    ServerConfig
	App       AppConfig
	Admin     AdminConfig
	Platforms PlatformsConfig
	Sites     map[string]SiteConfig
}

type fileConfig struct {
	ServerBlock    *ServerConfig             `json:"server"`
	Addr           string                    `json:"addr"`
	Port           string                    `json:"port"`
	AppBlock       *AppConfig                `json:"app"`
	PlatformsBlock *PlatformsConfig          `json:"platforms"`
	Sites          map[string]siteFileConfig `json:"sites"`
	AdminBlock     *AdminConfig              `json:"admin"`
	AdminConfig
}

type siteFileConfig struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Server      *ServerConfig  `json:"server"`
	App         *AppConfig     `json:"app"`
	YouTube     *YouTubeConfig `json:"youtube"`
	Twitch      *TwitchConfig  `json:"twitch"`
}

// Load reads the JSON config at the given path and returns the parsed structure.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var raw fileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	server := ServerConfig{
		Addr: raw.Addr,
		Port: raw.Port,
	}
	if raw.ServerBlock != nil {
		server = *raw.ServerBlock
		if server.Addr == "" {
			server.Addr = raw.Addr
		}
		if server.Port == "" {
			server.Port = raw.Port
		}
	}
	if server.Addr == "" {
		server.Addr = defaultAddr
	}
	if server.Port == "" {
		server.Port = defaultPort
	}

	admin := raw.AdminConfig
	if raw.AdminBlock != nil {
		admin = *raw.AdminBlock
	}
	if admin.TokenTTLSeconds <= 0 {
		admin.TokenTTLSeconds = 86400
	}

	app := AlertserverAppConfig()
	if raw.AppBlock != nil {
		app = *raw.AppBlock
	}
	if app.Templates == "" {
		app.Templates = defaultTemplatesDir
	}
	if app.Assets == "" {
		app.Assets = defaultAssetsDir
	}
	if app.Data == "" {
		app.Data = defaultData
	}
	if app.Name == "" {
		app.Name = alertserverName
	}

	// Parse global platforms config with environment fallbacks
	var platforms PlatformsConfig
	if raw.PlatformsBlock != nil {
		platforms = *raw.PlatformsBlock
	}
	// Apply environment variable fallbacks for global YouTube
	if platforms.YouTube.APIKey == "" || platforms.YouTube.APIKey == "YOUR_YOUTUBE_API_KEY_HERE" {
		platforms.YouTube.APIKey = youtubeAPIKeyFromEnv()
	}
	// Apply environment variable fallbacks for global Twitch
	if platforms.Twitch.ClientID == "" || platforms.Twitch.ClientID == "YOUR_TWITCH_CLIENT_ID_HERE" {
		platforms.Twitch.ClientID = twitchEnvValue("TWITCH_CLIENT_ID")
	}
	if platforms.Twitch.ClientSecret == "" || platforms.Twitch.ClientSecret == "YOUR_TWITCH_CLIENT_SECRET_HERE" {
		platforms.Twitch.ClientSecret = twitchEnvValue("TWITCH_CLIENT_SECRET")
	}
	if platforms.Twitch.EventSubSecret == "" || platforms.Twitch.EventSubSecret == "YOUR_TWITCH_EVENTSUB_SECRET_HERE" {
		platforms.Twitch.EventSubSecret = twitchEnvValue("TWITCH_EVENTSUB_SECRET")
	}

	sites := map[string]SiteConfig{}
	for key, site := range raw.Sites {
		siteServer := server
		if site.Server != nil {
			if site.Server.Addr != "" {
				siteServer.Addr = site.Server.Addr
			}
			if site.Server.Port != "" {
				siteServer.Port = site.Server.Port
			}
		}

		siteApp := app
		if site.App != nil {
			if site.App.Templates != "" {
				siteApp.Templates = site.App.Templates
			}
			if site.App.Assets != "" {
				siteApp.Assets = site.App.Assets
			}
			if site.App.Data != "" {
				siteApp.Data = site.App.Data
			}
			if site.App.Name != "" {
				siteApp.Name = site.App.Name
			}
		}

		siteName := site.Name
		if siteName == "" {
			siteName = siteApp.Name
			if siteName == "" {
				siteName = alertserverName
			}
		}

		// Build site-specific YouTube config: start from global, override with site-specific
		siteYouTube := platforms.YouTube // Start with global defaults
		if site.YouTube != nil {
			// Override with site-specific values
			if site.YouTube.Enabled != nil {
				siteYouTube.Enabled = site.YouTube.Enabled
			}
			if site.YouTube.CallbackURL != "" {
				siteYouTube.CallbackURL = site.YouTube.CallbackURL
			}
			if site.YouTube.LocalWebSubPath != "" {
				siteYouTube.LocalWebSubPath = site.YouTube.LocalWebSubPath
			}
			// Allow site-specific overrides of global settings if provided
			if site.YouTube.HubURL != "" {
				siteYouTube.HubURL = site.YouTube.HubURL
			}
			if site.YouTube.LeaseSeconds != 0 {
				siteYouTube.LeaseSeconds = site.YouTube.LeaseSeconds
			}
			if site.YouTube.Mode != "" {
				siteYouTube.Mode = site.YouTube.Mode
			}
			if site.YouTube.Verify != "" {
				siteYouTube.Verify = site.YouTube.Verify
			}
			if site.YouTube.APIKey != "" {
				siteYouTube.APIKey = site.YouTube.APIKey
			}
		}

		// Build site-specific Twitch config: start from global, override with site-specific
		siteTwitch := platforms.Twitch // Start with global defaults
		if site.Twitch != nil {
			// Override with site-specific values
			if site.Twitch.Enabled != nil {
				siteTwitch.Enabled = site.Twitch.Enabled
			}
			if site.Twitch.CallbackURL != "" {
				siteTwitch.CallbackURL = site.Twitch.CallbackURL
			}
			// Allow site-specific overrides of global settings if provided
			if site.Twitch.ClientID != "" {
				siteTwitch.ClientID = site.Twitch.ClientID
			}
			if site.Twitch.ClientSecret != "" {
				siteTwitch.ClientSecret = site.Twitch.ClientSecret
			}
			if site.Twitch.EventSubSecret != "" {
				siteTwitch.EventSubSecret = site.Twitch.EventSubSecret
			}
		}

		sites[key] = SiteConfig{
			Key:         key,
			Name:        siteName,
			Description: site.Description,
			Server:      siteServer,
			App:         siteApp,
			YouTube:     siteYouTube,
			Twitch:      siteTwitch,
		}
	}

	cfg := Config{
		Server:    server,
		App:       app,
		Admin:     admin,
		Platforms: platforms,
		Sites:     sites,
	}

	return cfg, nil
}

// MustLoad is a convenience wrapper around Load that panics on error.
func MustLoad(path string) Config {
	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}
	return cfg
}

// Save writes the configuration back to a JSON file at the given path.
func Save(cfg Config, path string) error {
	// Convert Config back to fileConfig format
	raw := fileConfig{
		ServerBlock: &cfg.Server,
		AppBlock: &AppConfig{
			Templates: cfg.App.Templates,
			Assets:    cfg.App.Assets,
			Data:      cfg.App.Data,
			Name:      cfg.App.Name,
		},
		PlatformsBlock: &cfg.Platforms,
		AdminBlock:     &cfg.Admin,
		Sites:          make(map[string]siteFileConfig),
	}

	// Convert sites - save site-specific overrides only
	for key, site := range cfg.Sites {
		// Create site-specific YouTube config (only site-specific fields)
		siteYouTube := &YouTubeConfig{
			Enabled:     site.YouTube.Enabled,
			CallbackURL: site.YouTube.CallbackURL,
		}
		// Create site-specific Twitch config (only site-specific fields)
		siteTwitch := &TwitchConfig{
			Enabled:     site.Twitch.Enabled,
			CallbackURL: site.Twitch.CallbackURL,
		}

		raw.Sites[key] = siteFileConfig{
			Name:        site.Name,
			Description: site.Description,
			Server:      &site.Server,
			App:         &site.App,
			YouTube:     siteYouTube,
			Twitch:      siteTwitch,
		}
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(raw, "", "\t")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// ResolveSite returns the combined configuration for the requested site. The
// empty site key resolves to the base configuration with global platform configs.
// The alertserver key returns the fallback alertserver configuration.
func ResolveSite(key string, cfg Config) (SiteConfig, error) {
	if key == "" {
		return SiteConfig{
			Key:         "",
			Name:        cfg.App.Name,
			Description: "",
			Server:      cfg.Server,
			App:         cfg.App,
			YouTube:     cfg.Platforms.YouTube,
			Twitch:      cfg.Platforms.Twitch,
		}, nil
	}

	// Handle the alertserver fallback site
	if key == AlertserverKey {
		return Alertserver(cfg), nil
	}

	site, ok := cfg.Sites[key]
	if !ok {
		return SiteConfig{}, fmt.Errorf("site %q not found", key)
	}
	return site, nil
}

// AllSites returns the list of configured sites only (no base site).
func AllSites(cfg Config) []SiteConfig {
	var sites []SiteConfig
	for key, site := range cfg.Sites {
		sites = append(sites, SiteConfig{
			Key:         key,
			Name:        site.Name,
			Description: site.Description,
			Server:      site.Server,
			App:         site.App,
			YouTube:     site.YouTube,
			Twitch:      site.Twitch,
		})
	}
	return sites
}

// DefaultConfig returns a configuration populated with default-site values. It
// is primarily used when loading config.json fails and the server needs a
// fallback site.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Addr: defaultAddr,
			Port: defaultPort,
		},
		App: AlertserverAppConfig(),
		Admin: AdminConfig{
			TokenTTLSeconds: 86400,
		},
		Platforms: PlatformsConfig{},
		Sites:     map[string]SiteConfig{},
	}
}

// AlertserverAppConfig returns the default fallback app configuration,
// including template, asset, and data roots.
func AlertserverAppConfig() AppConfig {
	return AppConfig{
		Name:      alertserverName,
		Templates: defaultTemplatesDir,
		Assets:    defaultAssetsDir,
		Data:      defaultData,
	}
}

// Alertserver returns a site configuration that points at the fallback assets
// and templates. Server listen values inherit from the provided config when
// present, otherwise the defaults are applied. Platform configs inherit from
// the global platforms config.
func Alertserver(cfg Config) SiteConfig {
	server := cfg.Server
	if strings.TrimSpace(server.Addr) == "" {
		server.Addr = defaultAddr
	}
	if strings.TrimSpace(server.Port) == "" {
		server.Port = defaultPort
	}
	return SiteConfig{
		Key:         AlertserverKey,
		Name:        alertserverName,
		Description: "Fallback site for multi-tenant streaming notifications",
		Server:      server,
		App:         AlertserverAppConfig(),
		YouTube:     cfg.Platforms.YouTube,
		Twitch:      cfg.Platforms.Twitch,
	}
}

func youtubeAPIKeyFromEnv() string {
	keys := []string{
		strings.TrimSpace(os.Getenv("YOUTUBE_API_KEY")),
		strings.TrimSpace(os.Getenv("YT_API_KEY")),
	}
	for _, key := range keys {
		if key != "" {
			return key
		}
	}
	return ""
}

func twitchEnvValue(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
