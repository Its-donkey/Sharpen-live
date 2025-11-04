package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/alerts"
	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/config"
	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/server"
	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/youtube"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	cfg = promptForRuntimeConfig(cfg)

	if err := cfg.Validate(); err != nil {
		log.Fatalf("configuration invalid: %v", err)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	checker := youtube.NewChecker(cfg.APIKey)
	monitor := alerts.NewMonitor(alerts.MonitorConfig{
		Checker:     checker,
		Interval:    cfg.PollInterval,
		Logger:      log.Default(),
		RootContext: rootCtx,
	})

	srv := server.New(server.Config{
		Processor: monitor,
		Logger:    log.Default(),
	})

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Routes(),
	}

	go func() {
		<-rootCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGracePeriod)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
		monitor.StopAll()
	}()

	log.Printf("Sharpen Live YouTube alert listener listening on %s", cfg.ListenAddr)
	if cfg.APIKey == "" {
		log.Println("warning: YOUTUBE_API_KEY not provided; live checks will fail until configured")
	}

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}

	// Give background goroutines time to shut down cleanly.
	time.Sleep(100 * time.Millisecond)
}

func promptForRuntimeConfig(cfg config.Config) config.Config {
	if !isInteractiveTerminal() {
		return cfg
	}

	reader := bufio.NewReader(os.Stdin)

	defaultPortDisplay := cfg.ListenAddr
	if strings.HasPrefix(defaultPortDisplay, ":") && len(defaultPortDisplay) > 1 {
		defaultPortDisplay = defaultPortDisplay[1:]
	}

	fmt.Printf("Enter port to listen on [%s]: ", defaultPortDisplay)
	portInput, _ := reader.ReadString('\n')
	portInput = strings.TrimSpace(portInput)
	if portInput != "" {
		if strings.HasPrefix(portInput, ":") || strings.Contains(portInput, ":") {
			cfg.ListenAddr = portInput
		} else {
			cfg.ListenAddr = fmt.Sprintf(":%s", portInput)
		}
	}

	maskedKey := maskSecret(cfg.APIKey)
	fmt.Printf("Enter YouTube API key [%s]: ", maskedKey)
	apiKeyInput, _ := reader.ReadString('\n')
	apiKeyInput = strings.TrimSpace(apiKeyInput)
	if apiKeyInput != "" {
		cfg.APIKey = apiKeyInput
	}

	return cfg
}

func isInteractiveTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == os.ModeCharDevice
}

func maskSecret(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unset"
	}
	if len(trimmed) <= 4 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:2] + strings.Repeat("*", len(trimmed)-4) + trimmed[len(trimmed)-2:]
}
