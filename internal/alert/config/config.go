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
	defaultLogs         = "data/logs/catch-all"
	defaultData         = "data/catch-all"
	defaultTemplatesDir = "ui/sites/catch-all/templates"
	defaultAssetsDir    = "ui/sites/catch-all"
	defaultSiteName     = "Catch-all"
	CatchAllSiteKey     = "catch-all"
)

// YouTubeConfig captures the WebSub-specific defaults persisted in config files.
type YouTubeConfig struct {
	HubURL       string `json:"hub_url"`
	CallbackURL  string `json:"callback_url"`
	LeaseSeconds int    `json:"lease_seconds"`
	Mode         string `json:"mode"`
	Verify       string `json:"verify"`
	APIKey       string `json:"api_key"`
}

// ServerConfig configures the HTTP listener used by alert-server.
type ServerConfig struct {
	Addr string `json:"addr"`
	Port string `json:"port"`
}

// AppConfig configures server-rendered assets/templates, log, and data locations.
type AppConfig struct {
	Templates string `json:"templates"`
	Assets    string `json:"assets"`
	Logs      string `json:"logs"`
	Data      string `json:"data"`
	Name      string `json:"name"`
}

// SiteConfig captures per-site overrides for server/app settings.
type SiteConfig struct {
	Key    string
	Name   string       `json:"name"`
	Server ServerConfig `json:"server"`
	App    AppConfig    `json:"app"`
}

// AdminConfig stores credentials for admin-authenticated APIs.
type AdminConfig struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	TokenTTLSeconds int    `json:"token_ttl_seconds"`
}

// Config represents the combined runtime settings parsed from config.json.
type Config struct {
	Server  ServerConfig
	App     AppConfig
	YouTube YouTubeConfig
	Admin   AdminConfig
	Sites   map[string]SiteConfig
}

type fileConfig struct {
	ServerBlock  *ServerConfig             `json:"server"`
	Addr         string                    `json:"addr"`
	Port         string                    `json:"port"`
	AppBlock     *AppConfig                `json:"app"`
	Sites        map[string]siteFileConfig `json:"sites"`
	YouTubeBlock *YouTubeConfig            `json:"youtube"`
	YouTubeConfig
	AdminBlock *AdminConfig `json:"admin"`
	AdminConfig
}

type siteFileConfig struct {
	Name   string        `json:"name"`
	Server *ServerConfig `json:"server"`
	App    *AppConfig    `json:"app"`
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

	yt := raw.YouTubeConfig
	if raw.YouTubeBlock != nil {
		yt = *raw.YouTubeBlock
	}
	if envAPIKey := youtubeAPIKeyFromEnv(); envAPIKey != "" {
		yt.APIKey = envAPIKey
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

	app := CatchAllAppConfig()
	if raw.AppBlock != nil {
		app = *raw.AppBlock
	}
	if app.Templates == "" {
		app.Templates = defaultTemplatesDir
	}
	if app.Assets == "" {
		app.Assets = defaultAssetsDir
	}
	if app.Logs == "" {
		app.Logs = defaultLogs
	}
	if app.Data == "" {
		app.Data = defaultData
	}
	if app.Name == "" {
		app.Name = defaultSiteName
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
		// Logs always use the base app logs path so every site writes to one location.
		siteApp.Logs = app.Logs

		siteName := site.Name
		if siteName == "" {
			siteName = siteApp.Name
			if siteName == "" {
				siteName = defaultSiteName
			}
		}

		sites[key] = SiteConfig{
			Key:    key,
			Name:   siteName,
			Server: siteServer,
			App:    siteApp,
		}
	}

	cfg := Config{
		Server:  server,
		App:     app,
		YouTube: yt,
		Admin:   admin,
		Sites:   sites,
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

// ResolveSite returns the combined configuration for the requested site. The
// empty site key resolves to the base (Sharpen.Live) configuration.
func ResolveSite(key string, cfg Config) (SiteConfig, error) {
	if key == "" {
		return SiteConfig{
			Key:    "",
			Name:   cfg.App.Name,
			Server: cfg.Server,
			App:    cfg.App,
		}, nil
	}

	site, ok := cfg.Sites[key]
	if !ok {
		return SiteConfig{}, fmt.Errorf("site %q not found", key)
	}
	return site, nil
}

// AllSites returns the list of configured sites, including the base
// Sharpen.Live site.
func AllSites(cfg Config) []SiteConfig {
	sites := []SiteConfig{{
		Key:    "",
		Name:   cfg.App.Name,
		Server: cfg.Server,
		App:    cfg.App,
	}}
	for key, site := range cfg.Sites {
		sites = append(sites, SiteConfig{
			Key:    site.Key,
			Name:   site.Name,
			Server: site.Server,
			App:    site.App,
		})
		sites[len(sites)-1].Key = key
	}
	return sites
}

// DefaultConfig returns a configuration populated with catch-all defaults. It
// is primarily used when loading config.json fails and the server needs a
// fallback site.
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Addr: defaultAddr,
			Port: defaultPort,
		},
		App: CatchAllAppConfig(),
		YouTube: YouTubeConfig{
			HubURL:       "",
			CallbackURL:  "",
			LeaseSeconds: 0,
			Verify:       "",
			APIKey:       "",
		},
		Admin: AdminConfig{
			TokenTTLSeconds: 86400,
		},
		Sites: map[string]SiteConfig{},
	}
}

// CatchAllAppConfig returns the default catch-all app configuration, including
// template, asset, log, and data roots.
func CatchAllAppConfig() AppConfig {
	return AppConfig{
		Name:      defaultSiteName,
		Templates: defaultTemplatesDir,
		Assets:    defaultAssetsDir,
		Logs:      defaultLogs,
		Data:      defaultData,
	}
}

// CatchAllSite returns a site configuration that points at the catch-all assets
// and templates. Server listen values inherit from the provided config when
// present, otherwise the defaults are applied.
func CatchAllSite(cfg Config) SiteConfig {
	server := cfg.Server
	if strings.TrimSpace(server.Addr) == "" {
		server.Addr = defaultAddr
	}
	if strings.TrimSpace(server.Port) == "" {
		server.Port = defaultPort
	}
	return SiteConfig{
		Key:    CatchAllSiteKey,
		Name:   defaultSiteName,
		Server: server,
		App:    CatchAllAppConfig(),
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
