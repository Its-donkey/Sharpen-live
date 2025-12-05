package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_LoadDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)

	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !settings.YouTubeEnabled {
		t.Error("Expected YouTubeEnabled to default to true")
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)

	original := Settings{
		YouTubeEnabled: false,
	}

	if err := store.Save(original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.YouTubeEnabled != original.YouTubeEnabled {
		t.Errorf("YouTubeEnabled = %v, want %v", loaded.YouTubeEnabled, original.YouTubeEnabled)
	}
}

func TestStore_SetYouTubeEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)

	if err := store.SetYouTubeEnabled(false); err != nil {
		t.Fatalf("SetYouTubeEnabled() error = %v", err)
	}

	if store.IsYouTubeEnabled() {
		t.Error("Expected IsYouTubeEnabled() to return false")
	}

	if err := store.SetYouTubeEnabled(true); err != nil {
		t.Fatalf("SetYouTubeEnabled() error = %v", err)
	}

	if !store.IsYouTubeEnabled() {
		t.Error("Expected IsYouTubeEnabled() to return true")
	}
}

func TestStore_IsYouTubeEnabled_DefaultOnError(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Write invalid JSON
	if err := os.WriteFile(settingsPath, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store := New(tmpDir)

	// Should default to true on error
	if !store.IsYouTubeEnabled() {
		t.Error("Expected IsYouTubeEnabled() to default to true on error")
	}
}

func TestStore_Path(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)

	expected := filepath.Join(tmpDir, "settings.json")
	if store.Path() != expected {
		t.Errorf("Path() = %v, want %v", store.Path(), expected)
	}
}
