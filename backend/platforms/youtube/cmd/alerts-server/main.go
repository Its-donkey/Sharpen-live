package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
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
