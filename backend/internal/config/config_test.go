package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFromEnvWithOverrides(t *testing.T) {
	tmp := t.TempDir()

	dataDir := filepath.Join(tmp, "custom-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}

	streamersFile := filepath.Join(tmp, "streamers.json")
	submissionsFile := filepath.Join(tmp, "submissions.json")
	staticDir := filepath.Join(tmp, "public")

	t.Setenv(envListenAddr, "127.0.0.1:9090")
	t.Setenv(envAdminToken, "token")
	t.Setenv(envAdminEmail, "admin@example.com")
	t.Setenv(envAdminPassword, "secret")
	t.Setenv(envDataDir, dataDir)
	t.Setenv(envStreamersFile, streamersFile)
	t.Setenv(envSubmissionsFile, submissionsFile)
	t.Setenv(envStaticDir, staticDir)
	t.Setenv(envDatabaseURL, "postgres://localhost/test?sslmode=disable")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}

	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Fatalf("expected listen addr override, got %q", cfg.ListenAddr)
	}
	if cfg.StreamersPath != streamersFile {
		t.Fatalf("expected streamers path %q, got %q", streamersFile, cfg.StreamersPath)
	}
	if cfg.SubmissionsPath != submissionsFile {
		t.Fatalf("expected submissions path %q, got %q", submissionsFile, cfg.SubmissionsPath)
	}
	if cfg.StaticDir != staticDir {
		t.Fatalf("expected static dir %q, got %q", staticDir, cfg.StaticDir)
	}
	if cfg.DataDir != dataDir {
		t.Fatalf("expected data dir %q, got %q", dataDir, cfg.DataDir)
	}
	if cfg.DatabaseURL != "postgres://localhost/test?sslmode=disable" {
		t.Fatalf("expected database url override, got %q", cfg.DatabaseURL)
	}
}

func TestFromEnvDetectsDataDir(t *testing.T) {
	tmp := t.TempDir()
	projectRoot := filepath.Join(tmp, "project")
	backendRoot := filepath.Join(projectRoot, "backend")
	if err := os.MkdirAll(filepath.Join(backendRoot, "data"), 0o755); err != nil {
		t.Fatalf("create backend data dir: %v", err)
	}

	cwd := filepath.Join(backendRoot, "cmd")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("create cmd directory: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(tmp, "data"), 0o755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv(envAdminToken, "token")
	t.Setenv(envAdminEmail, "admin@example.com")
	t.Setenv(envAdminPassword, "secret")
	t.Setenv(envListenAddr, "")
	t.Setenv(envPort, "8088")
	t.Setenv(envDatabaseURL, "postgres://localhost/test?sslmode=disable")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}

	expectedDataDir := filepath.Join(backendRoot, "data")
	detectedDir := filepath.Dir(cfg.StreamersPath)

	resolvedExpected, err := filepath.EvalSymlinks(expectedDataDir)
	if err != nil {
		t.Fatalf("eval expected: %v", err)
	}
	resolvedDetected, err := filepath.EvalSymlinks(detectedDir)
	if err != nil {
		t.Fatalf("eval detected: %v", err)
	}

	if resolvedDetected != resolvedExpected {
		t.Fatalf("expected detected data dir %q, got %q", resolvedExpected, resolvedDetected)
	}
	if cfg.ListenAddr != ":8088" {
		t.Fatalf("expected listen addr :8088, got %q", cfg.ListenAddr)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := Config{
		ListenAddr:      ":8080",
		AdminToken:      "token",
		AdminEmail:      "admin@example.com",
		AdminPassword:   "secret",
		DatabaseURL:     "postgres://localhost/test?sslmode=disable",
		DataDir:         "data",
		StreamersPath:   "streamers.json",
		SubmissionsPath: "submissions.json",
		StaticDir:       "static",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error validating config: %v", err)
	}

	invalidCases := []struct {
		name string
		mut  func(*Config)
	}{
		{"listen", func(c *Config) { c.ListenAddr = "" }},
		{"streamers", func(c *Config) { c.StreamersPath = "" }},
		{"submissions", func(c *Config) { c.SubmissionsPath = "" }},
		{"static", func(c *Config) { c.StaticDir = "" }},
		{"dataDir", func(c *Config) { c.DataDir = "" }},
		{"token", func(c *Config) { c.AdminToken = "" }},
		{"email", func(c *Config) { c.AdminEmail = "" }},
		{"password", func(c *Config) { c.AdminPassword = "" }},
		{"database", func(c *Config) { c.DatabaseURL = "" }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			copyCfg := cfg
			tc.mut(&copyCfg)
			if err := copyCfg.Validate(); err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
		})
	}
}

func TestDetectDataDirFallback(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "data"), 0o755); err != nil {
		t.Fatalf("create fallback data dir: %v", err)
	}
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	detected := detectDataDir()
	resolvedDetected, err := filepath.EvalSymlinks(detected)
	if err != nil {
		t.Fatalf("eval detected: %v", err)
	}
	resolvedTmp, err := filepath.EvalSymlinks(filepath.Join(tmp, "data"))
	if err != nil {
		t.Fatalf("eval tmp: %v", err)
	}
	if resolvedDetected != resolvedTmp {
		t.Fatalf("expected fallback path %q, got %q", resolvedTmp, resolvedDetected)
	}
}
