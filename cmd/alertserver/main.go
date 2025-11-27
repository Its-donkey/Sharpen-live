package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	uiserver "github.com/Its-donkey/Sharpen-live/internal/ui/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
		// If a second signal arrives, force exit immediately.
		<-sigCh
		log.Println("second interrupt received, forcing shutdown")
		os.Exit(1)
	}()
	defer func() {
		signal.Stop(sigCh)
		cancel()
	}()

	listen := flag.String("listen", "", "address to serve the Sharpen.Live UI (defaults to config.json server.addr+port)")
	templatesDir := flag.String("templates", "", "path to the html/template files (defaults to config.json ui.templates)")
	assetsDir := flag.String("assets", "", "path where styles.css is located (defaults to config.json app.assets)")
	logDir := flag.String("logs", "", "directory for category logs (defaults to config.json app.logs)")
	dataDir := flag.String("data", "", "directory for data files (streamers/submissions); defaults to config.json app.data")
	configPath := flag.String("config", "config.json", "path to server configuration")
	site := flag.String("site", "", "site key to serve (defaults to all configured sites when empty)")
	flag.Parse()

	loadedConfig, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	rawSite := strings.TrimSpace(*site)
	siteRequested := rawSite != ""
	normalizedSite := normalizeSiteKey(rawSite, loadedConfig.App.Name)

	siteKeys := []string{}
	if siteRequested {
		if _, err := config.ResolveSite(normalizedSite, loadedConfig); err != nil {
			log.Fatalf("invalid site %q: %v", *site, err)
		}
		siteKeys = append(siteKeys, normalizedSite)
	} else {
		for _, resolved := range config.AllSites(loadedConfig) {
			siteKeys = append(siteKeys, resolved.Key)
		}
	}

	if len(siteKeys) > 1 {
		if *listen != "" || *templatesDir != "" || *assetsDir != "" || *logDir != "" || *dataDir != "" {
			log.Fatal("path/listen overrides require -site to target a single site")
		}
	}

	type runResult struct {
		site string
		err  error
	}

	results := make(chan runResult, len(siteKeys))
	for _, siteKey := range siteKeys {
		cfg := uiserver.Options{
			Listen:       *listen,
			TemplatesDir: *templatesDir,
			AssetsDir:    *assetsDir,
			LogDir:       *logDir,
			DataDir:      *dataDir,
			ConfigPath:   *configPath,
			Site:         siteKey,
		}
		go func(key string) {
			results <- runResult{site: key, err: uiserver.Run(ctx, cfg)}
		}(siteKey)
	}

	var firstErr error
	var failingSite string
	for range siteKeys {
		res := <-results
		if res.err != nil && !errors.Is(res.err, context.Canceled) && firstErr == nil {
			firstErr = res.err
			failingSite = res.site
			cancel()
		}
	}

	if firstErr != nil {
		log.Fatalf("server error (site %s): %v", failingSite, firstErr)
	}
}

func normalizeSiteKey(siteArg, baseName string) string {
	key := strings.TrimSpace(siteArg)
	if key == "" {
		return ""
	}

	if normalizeNameKey(key) == normalizeNameKey(baseName) {
		return ""
	}

	switch strings.ToLower(key) {
	case "default", "base":
		return ""
	}

	return key
}

func normalizeNameKey(name string) string {
	replacer := strings.NewReplacer(".", "-", "_", "-", " ", "-")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(name)))
}
