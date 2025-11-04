package settings

import (
	"context"
	"errors"
)

// ErrNotFound indicates the settings record does not exist yet.
var ErrNotFound = errors.New("settings: not found")

// Settings captures configurable site settings persisted to the database.
type Settings struct {
	AdminToken                string `json:"adminToken"`
	AdminEmail                string `json:"adminEmail"`
	AdminPassword             string `json:"adminPassword"`
	YouTubeAPIKey             string `json:"youtubeApiKey"`
	YouTubeAlertsCallback     string `json:"youtubeAlertsCallback"`
	YouTubeAlertsSecret       string `json:"youtubeAlertsSecret"`
	YouTubeAlertsVerifyPrefix string `json:"youtubeAlertsVerifyPrefix"`
	YouTubeAlertsVerifySuffix string `json:"youtubeAlertsVerifySuffix"`
	YouTubeAlertsHubURL       string `json:"youtubeAlertsHubUrl"`
	ListenAddr                string `json:"listenAddr"`
	DataDir                   string `json:"dataDir"`
	StaticDir                 string `json:"staticDir"`
	StreamersFile             string `json:"streamersFile"`
	SubmissionsFile           string `json:"submissionsFile"`
}

// Store defines persistence operations for settings.
type Store interface {
	EnsureSchema(ctx context.Context) error
	Load(ctx context.Context) (Settings, error)
	Save(ctx context.Context, settings Settings) error
}
