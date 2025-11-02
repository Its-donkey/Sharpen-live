package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)


const (
	defaultListenAddr      = ":8880"
	defaultDataDir         = "backend/data"
	defaultStreamersFile   = "streamers.json"
	defaultSubmissionsFile = "submissions.json"
	defaultStaticDir       = "frontend/dist"

	envListenAddr      = "LISTEN_ADDR"
	envPort            = "PORT"
	envAdminToken      = "ADMIN_TOKEN"
	envAdminEmail      = "ADMIN_EMAIL"
	envAdminPassword   = "ADMIN_PASSWORD"
	envDataDir         = "SHARPEN_DATA_DIR"
	envStreamersFile   = "SHARPEN_STREAMERS_FILE"
	envSubmissionsFile = "SHARPEN_SUBMISSIONS_FILE"
	envStaticDir       = "SHARPEN_STATIC_DIR"
)

// Config captures runtime settings for the Sharpen Live API server.
type Config struct {
	ListenAddr      string
	AdminToken      string
	AdminEmail      string
	AdminPassword   string
	StreamersPath   string
	SubmissionsPath string
	StaticDir       string
}

// FromEnv constructs a Config by reading environment variables with defaults.
func FromEnv() (Config, error) {
	cfg := Config{
		ListenAddr: defaultListenAddr,
		StaticDir:  defaultStaticDir,
	}

	if v := strings.TrimSpace(os.Getenv(envListenAddr)); v != "" {
		cfg.ListenAddr = v
	} else if port := strings.TrimSpace(os.Getenv(envPort)); port != "" {
		cfg.ListenAddr = fmt.Sprintf(":%s", port)
	}

	cfg.AdminToken = strings.TrimSpace(os.Getenv(envAdminToken))
	cfg.AdminEmail = strings.TrimSpace(os.Getenv(envAdminEmail))
	cfg.AdminPassword = strings.TrimSpace(os.Getenv(envAdminPassword))

	dataDir := strings.TrimSpace(os.Getenv(envDataDir))
	if dataDir == "" {
		dataDir = detectDataDir()
	}

	streamersPath := strings.TrimSpace(os.Getenv(envStreamersFile))
	if streamersPath == "" {
		streamersPath = filepath.Join(dataDir, defaultStreamersFile)
	}
	cfg.StreamersPath = streamersPath

	submissionsPath := strings.TrimSpace(os.Getenv(envSubmissionsFile))
	if submissionsPath == "" {
		submissionsPath = filepath.Join(dataDir, defaultSubmissionsFile)
	}
	cfg.SubmissionsPath = submissionsPath

	if v := strings.TrimSpace(os.Getenv(envStaticDir)); v != "" {
		cfg.StaticDir = v
	}

	return cfg, nil
}

// Validate ensures the configuration is usable.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return fmt.Errorf("config: listen address is required")
	}
	if strings.TrimSpace(c.StreamersPath) == "" {
		return fmt.Errorf("config: streamers path is required")
	}
	if strings.TrimSpace(c.SubmissionsPath) == "" {
		return fmt.Errorf("config: submissions path is required")
	}
	if strings.TrimSpace(c.StaticDir) == "" {
		return fmt.Errorf("config: static directory is required")
	}
	if c.AdminToken == "" {
		return fmt.Errorf("config: admin token is required")
	}
	if c.AdminEmail == "" {
		return fmt.Errorf("config: admin email is required")
	}
	if c.AdminPassword == "" {
		return fmt.Errorf("config: admin password is required")
	}
	return nil
}

func detectDataDir() string {
	candidates := []string{
		"data",
		filepath.Join("api", "data"),
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	var roots []string
	for dir := wd; dir != ""; dir = filepath.Dir(dir) {
		roots = append(roots, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}

	for _, base := range roots {
		for _, candidate := range candidates {
			path := filepath.Join(base, candidate)
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				return path
			}
		}
	}

	// Fallback to first candidate relative to working directory.
	return filepath.Join(wd, candidates[0])
}
