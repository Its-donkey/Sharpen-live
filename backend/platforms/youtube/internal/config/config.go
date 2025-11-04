package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultListenAddr        = ":8080"
	defaultPollInterval      = 5 * time.Minute
	defaultShutdownGrace     = 10 * time.Second
	defaultStreamersPath     = "backend/data/streamers.json"
	envYouTubeAPIKey         = "YOUTUBE_API_KEY"
	envStreamersPath         = "STREAMERS_JSON"
	envListenAddr            = "LISTEN_ADDR"
	envPort                  = "YTPORT"
	envPollInterval          = "POLL_INTERVAL"
	envShutdownGraceDuration = "SHUTDOWN_GRACE_PERIOD"
	envDatabaseURL           = "DATABASE_URL"
)

// Config captures runtime settings for the YouTube alert listener.
type Config struct {
	ListenAddr          string
	APIKey              string
	StreamersPath       string
	DatabaseURL         string
	PollInterval        time.Duration
	ShutdownGracePeriod time.Duration
}

// FromEnv loads configuration values from environment variables, using sensible defaults.
func FromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:          defaultListenAddr,
		PollInterval:        defaultPollInterval,
		ShutdownGracePeriod: defaultShutdownGrace,
		APIKey:              os.Getenv(envYouTubeAPIKey),
		StreamersPath:       defaultStreamersPath,
		DatabaseURL:         strings.TrimSpace(os.Getenv(envDatabaseURL)),
	}

	if v := strings.TrimSpace(os.Getenv(envStreamersPath)); v != "" {
		cfg.StreamersPath = v
	}

	if v := strings.TrimSpace(os.Getenv(envListenAddr)); v != "" {
		cfg.ListenAddr = v
	} else if port := strings.TrimSpace(os.Getenv(envPort)); port != "" {
		cfg.ListenAddr = fmt.Sprintf(":%s", port)
	}

	if v := strings.TrimSpace(os.Getenv(envPollInterval)); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", envPollInterval, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("%s must be positive", envPollInterval)
		}
		cfg.PollInterval = d
	}

	if v := strings.TrimSpace(os.Getenv(envShutdownGraceDuration)); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", envShutdownGraceDuration, err)
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("%s must be positive", envShutdownGraceDuration)
		}
		cfg.ShutdownGracePeriod = d
	}

	return cfg, nil
}

// Validate ensures required configuration values are present before the application starts.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return errors.New("config: listen address is required")
	}

	if strings.TrimSpace(c.StreamersPath) == "" {
		return errors.New("config: streamers json path is required")
	}

	if c.PollInterval <= 0 {
		return errors.New("config: poll interval must be positive")
	}

	if c.ShutdownGracePeriod <= 0 {
		return errors.New("config: shutdown grace period must be positive")
	}

	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("config: database url is required")
	}

	return nil
}
