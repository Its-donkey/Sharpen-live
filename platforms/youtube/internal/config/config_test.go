package config

import (
	"os"
	"testing"
	"time"
)

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "")
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("PORT", "")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("SHUTDOWN_GRACE_PERIOD", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.ListenAddr != defaultListenAddr {
		t.Errorf("expected listen addr %q, got %q", defaultListenAddr, cfg.ListenAddr)
	}

	if cfg.PollInterval != defaultPollInterval {
		t.Errorf("unexpected poll interval: %v", cfg.PollInterval)
	}

	if cfg.ShutdownGracePeriod != defaultShutdownGrace {
		t.Errorf("unexpected shutdown grace period: %v", cfg.ShutdownGracePeriod)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "secret")
	t.Setenv("LISTEN_ADDR", "127.0.0.1:9090")
	t.Setenv("POLL_INTERVAL", "1m30s")
	t.Setenv("SHUTDOWN_GRACE_PERIOD", "5s")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.APIKey != "secret" {
		t.Errorf("expected api key override")
	}

	if cfg.ListenAddr != "127.0.0.1:9090" {
		t.Errorf("unexpected listen addr: %s", cfg.ListenAddr)
	}

	if cfg.PollInterval != 90*time.Second {
		t.Errorf("unexpected poll interval: %v", cfg.PollInterval)
	}

	if cfg.ShutdownGracePeriod != 5*time.Second {
		t.Errorf("unexpected shutdown grace: %v", cfg.ShutdownGracePeriod)
	}
}

func TestFromEnvPortFallback(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("PORT", "5050")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.ListenAddr != ":5050" {
		t.Errorf("expected listen addr :5050, got %s", cfg.ListenAddr)
	}
}

func TestFromEnvInvalidDurations(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "abc")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error for invalid poll interval")
	}

	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("SHUTDOWN_GRACE_PERIOD", "0s")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error for non-positive shutdown grace period")
	}
}

func TestValidate(t *testing.T) {
	cfg := Config{
		ListenAddr:          "",
		PollInterval:        time.Minute,
		ShutdownGracePeriod: time.Second,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty listen addr")
	}

	cfg.ListenAddr = ":8080"
	cfg.PollInterval = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for non-positive poll interval")
	}

	cfg.PollInterval = time.Minute
	cfg.ShutdownGracePeriod = -time.Second
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid shutdown grace period")
	}

	cfg.ShutdownGracePeriod = time.Second
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Ensure FromEnv does not retain leaked environment variables between tests.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
