package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Its-donkey/Sharpen-live/backend/internal/api"
	"github.com/Its-donkey/Sharpen-live/backend/internal/config"
	"github.com/Its-donkey/Sharpen-live/backend/internal/settings"
	"github.com/Its-donkey/Sharpen-live/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		logger.Error("validate config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	settingsStore := settings.NewPostgresStore(pool)
	if err := settingsStore.EnsureSchema(ctx); err != nil {
		logger.Error("ensure settings schema", "error", err)
		os.Exit(1)
	}

	initialSettings, err := settingsStore.Load(ctx)
	if errors.Is(err, settings.ErrNotFound) {
		initialSettings = settings.Settings{
			AdminToken:                cfg.AdminToken,
			AdminEmail:                cfg.AdminEmail,
			AdminPassword:             cfg.AdminPassword,
			YouTubeAPIKey:             cfg.YouTubeAPIKey,
			YouTubeAlertsCallback:     cfg.YouTubeAlertsCallback,
			YouTubeAlertsSecret:       cfg.YouTubeAlertsSecret,
			YouTubeAlertsVerifyPrefix: cfg.YouTubeAlertsVerifyPrefix,
			YouTubeAlertsVerifySuffix: cfg.YouTubeAlertsVerifySuffix,
			YouTubeAlertsHubURL:       cfg.YouTubeAlertsHubURL,
			ListenAddr:                cfg.ListenAddr,
			DataDir:                   cfg.DataDir,
			StaticDir:                 cfg.StaticDir,
			StreamersFile:             cfg.StreamersPath,
			SubmissionsFile:           cfg.SubmissionsPath,
		}
		if err := settingsStore.Save(ctx, initialSettings); err != nil {
			logger.Error("seed settings", "error", err)
			os.Exit(1)
		}
	} else if err != nil {
		logger.Error("load settings", "error", err)
		os.Exit(1)
	} else {
		initialSettings = mergeSettingsWithConfig(initialSettings, cfg)
		if err := settingsStore.Save(ctx, initialSettings); err != nil {
			logger.Error("sync settings", "error", err)
			os.Exit(1)
		}
	}

	listenAddr := cfg.ListenAddr
	if strings.TrimSpace(initialSettings.ListenAddr) != "" {
		listenAddr = initialSettings.ListenAddr
	}
	staticDir := cfg.StaticDir
	if strings.TrimSpace(initialSettings.StaticDir) != "" {
		staticDir = initialSettings.StaticDir
	}
	streamersPath := cfg.StreamersPath
	if strings.TrimSpace(initialSettings.StreamersFile) != "" {
		streamersPath = initialSettings.StreamersFile
	}
	submissionsPath := cfg.SubmissionsPath
	if strings.TrimSpace(initialSettings.SubmissionsFile) != "" {
		submissionsPath = initialSettings.SubmissionsFile
	}

	store, err := storage.NewJSONStore(streamersPath, submissionsPath)
	if err != nil {
		logger.Error("init store", "error", err)
		os.Exit(1)
	}

	// Ensure persisted settings use the final file paths.
	initialSettings.StreamersFile = streamersPath
	initialSettings.SubmissionsFile = submissionsPath
	initialSettings.StaticDir = staticDir
	initialSettings.ListenAddr = listenAddr
	if err := settingsStore.Save(ctx, initialSettings); err != nil {
		logger.Error("finalize settings", "error", err)
		os.Exit(1)
	}

	srv := api.New(store, settingsStore, initialSettings)
	staticHandler := spaHandler(staticDir)

	httpServer := &http.Server{
		Addr:    listenAddr,
		Handler: srv.Handler(staticHandler),
		BaseContext: func(_ net.Listener) context.Context {
			return context.Background()
		},
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server.started", "addr", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server.failed", "error", err)
			os.Exit(1)
		}
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-shutdownCtx.Done()

	logger.Info("server.shutting_down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("server.shutdown_error", "error", err)
	} else {
		logger.Info("server.stopped")
	}
}

func mergeSettingsWithConfig(current settings.Settings, cfg config.Config) settings.Settings {
	if strings.TrimSpace(current.AdminToken) == "" {
		current.AdminToken = cfg.AdminToken
	}
	if strings.TrimSpace(current.AdminEmail) == "" {
		current.AdminEmail = cfg.AdminEmail
	}
	if strings.TrimSpace(current.AdminPassword) == "" {
		current.AdminPassword = cfg.AdminPassword
	}
	if strings.TrimSpace(current.YouTubeAPIKey) == "" {
		current.YouTubeAPIKey = cfg.YouTubeAPIKey
	}
	if strings.TrimSpace(current.YouTubeAlertsCallback) == "" {
		current.YouTubeAlertsCallback = cfg.YouTubeAlertsCallback
	}
	if strings.TrimSpace(current.YouTubeAlertsSecret) == "" {
		current.YouTubeAlertsSecret = cfg.YouTubeAlertsSecret
	}
	if strings.TrimSpace(current.YouTubeAlertsVerifyPrefix) == "" {
		current.YouTubeAlertsVerifyPrefix = cfg.YouTubeAlertsVerifyPrefix
	}
	if strings.TrimSpace(current.YouTubeAlertsVerifySuffix) == "" {
		current.YouTubeAlertsVerifySuffix = cfg.YouTubeAlertsVerifySuffix
	}
	if strings.TrimSpace(current.YouTubeAlertsHubURL) == "" {
		current.YouTubeAlertsHubURL = cfg.YouTubeAlertsHubURL
	}
	if strings.TrimSpace(current.ListenAddr) == "" {
		current.ListenAddr = cfg.ListenAddr
	}
	if strings.TrimSpace(current.DataDir) == "" {
		current.DataDir = cfg.DataDir
	}
	if strings.TrimSpace(current.StaticDir) == "" {
		current.StaticDir = cfg.StaticDir
	}
	if strings.TrimSpace(current.StreamersFile) == "" {
		current.StreamersFile = cfg.StreamersPath
	}
	if strings.TrimSpace(current.SubmissionsFile) == "" {
		current.SubmissionsFile = cfg.SubmissionsPath
	}
	return current
}

func spaHandler(staticDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleaned := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		requestPath := filepath.Join(staticDir, cleaned)

		if !strings.HasPrefix(requestPath, staticDir) {
			http.NotFound(w, r)
			return
		}

		if info, err := os.Stat(requestPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		indexPath := filepath.Join(staticDir, "index.html")
		data, err := os.ReadFile(indexPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to load application: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}
