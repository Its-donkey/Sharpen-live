package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/alerts"
	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/config"
	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/logstore"
	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/server"
	"github.com/Its-donkey/Sharpen-live/backend/platforms/youtube/internal/youtube"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("configuration invalid: %v", err)
	}

	baseCtx := context.Background()
	pool, err := pgxpool.New(baseCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	store := logstore.New(pool)
	if err := store.EnsureSchema(baseCtx); err != nil {
		log.Fatalf("ensure log schema: %v", err)
	}

	logWriter := logstore.NewWriter(store)
	combinedWriter := io.MultiWriter(os.Stdout, logWriter)
	log.SetOutput(combinedWriter)
	logger := log.New(combinedWriter, "", log.LstdFlags)

	rootCtx, stop := signal.NotifyContext(baseCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	checker := youtube.NewChecker(cfg.APIKey)
	monitor := alerts.NewMonitor(alerts.MonitorConfig{
		Checker:     checker,
		Interval:    cfg.PollInterval,
		Logger:      logger,
		RootContext: rootCtx,
	})

	srv := server.New(server.Config{
		Processor: monitor,
		Logger:    logger,
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
			logger.Printf("graceful shutdown failed: %v", err)
		}
		monitor.StopAll()
	}()

	logger.Printf("Sharpen Live YouTube alert listener listening on %s", cfg.ListenAddr)
	if cfg.APIKey == "" {
		logger.Println("warning: YOUTUBE_API_KEY not provided; live checks will fail until configured")
	}

	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("server error: %v", err)
	}

	// Give background goroutines time to shut down cleanly.
	time.Sleep(100 * time.Millisecond)
}
