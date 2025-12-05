// Package settings provides site-specific configuration management.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Settings represents site-specific runtime configuration.
type Settings struct {
	YouTubeEnabled bool `json:"youtube_enabled"`
}

// Store manages site-specific settings persistence.
type Store struct {
	path string
	mu   sync.RWMutex
}

// New creates a new settings store for the specified data directory.
func New(dataDir string) *Store {
	return &Store{
		path: filepath.Join(dataDir, "settings.json"),
	}
}

// Load reads settings from disk, returning defaults if file doesn't exist.
func (s *Store) Load() (Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		// Return default settings if file doesn't exist
		return Settings{YouTubeEnabled: true}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read settings: %w", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, fmt.Errorf("decode settings: %w", err)
	}

	return settings, nil
}

// Save writes settings to disk.
func (s *Store) Save(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
}

// SetYouTubeEnabled updates the YouTube enabled setting.
func (s *Store) SetYouTubeEnabled(enabled bool) error {
	settings, err := s.Load()
	if err != nil {
		return err
	}
	settings.YouTubeEnabled = enabled
	return s.Save(settings)
}

// IsYouTubeEnabled returns whether YouTube integration is enabled for this site.
func (s *Store) IsYouTubeEnabled() bool {
	settings, err := s.Load()
	if err != nil {
		// Default to enabled if we can't read settings
		return true
	}
	return settings.YouTubeEnabled
}

// Path returns the file path where settings are stored.
func (s *Store) Path() string {
	return s.path
}
