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

	"github.com/Its-donkey/Sharpen-live/api/internal/config"
	"github.com/Its-donkey/Sharpen-live/api/internal/server"
	"github.com/Its-donkey/Sharpen-live/api/internal/storage"
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

	store, err := storage.NewJSONStore(cfg.StreamersPath, cfg.SubmissionsPath)
	if err != nil {
		logger.Error("init store", "error", err)
		os.Exit(1)
	}

	srv := server.New(store, cfg.AdminToken)
	staticHandler := spaHandler(cfg.StaticDir)

	httpServer := &http.Server{
		Addr:    cfg.ListenAddr,
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

func spaHandler(staticDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := filepath.Join(staticDir, filepath.Clean(strings.TrimPrefix(r.URL.Path, "/")))

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
