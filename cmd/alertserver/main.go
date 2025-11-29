package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	uiserver "github.com/Its-donkey/Sharpen-live/internal/ui/server"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
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

	loadedConfig, configErr := config.Load(*configPath)
	fallbackErrors := []string{}
	if configErr != nil {
		fallbackErrors = append(fallbackErrors, fmt.Sprintf("failed to load config: %v", configErr))
		loadedConfig = config.DefaultConfig()
	}

	rawSite := strings.TrimSpace(*site)
	siteRequested := rawSite != ""
	normalizedSite := config.NormaliseSiteKey(rawSite)

	type siteTarget struct {
		cfg    config.SiteConfig
		errors []string
	}

	siteTargets := []siteTarget{}
	switch {
	case configErr != nil && !siteRequested:
		siteTargets = append(siteTargets, siteTarget{
			cfg:    config.CatchAllSite(loadedConfig),
			errors: fallbackErrors,
		})
	case siteRequested:
		siteCfg, err := config.ResolveSite(normalizedSite, loadedConfig)
		errors := append([]string{}, fallbackErrors...)
		if err != nil {
			errors = append(errors, fmt.Sprintf("site %q not found; serving %s", rawSite, config.CatchAllSiteKey))
			siteCfg = config.CatchAllSite(loadedConfig)
		}
		siteTargets = append(siteTargets, siteTarget{
			cfg:    siteCfg,
			errors: errors,
		})
	default:
		for _, resolved := range config.AllSites(loadedConfig) {
			siteTargets = append(siteTargets, siteTarget{
				cfg:    resolved,
				errors: append([]string{}, fallbackErrors...),
			})
		}
	}

	if len(siteTargets) == 0 {
		log.Fatal("no site configurations found")
	}

	if len(siteTargets) > 1 {
		if *listen != "" || *templatesDir != "" || *assetsDir != "" || *logDir != "" || *dataDir != "" {
			log.Fatal("path/listen overrides require -site to target a single site")
		}
	}

	type runResult struct {
		site string
		err  error
	}

	results := make(chan runResult, len(siteTargets))
	for _, target := range siteTargets {
		cfg := uiserver.Options{
			Listen:         *listen,
			TemplatesDir:   *templatesDir,
			AssetsDir:      *assetsDir,
			LogDir:         *logDir,
			DataDir:        *dataDir,
			ConfigPath:     *configPath,
			Site:           target.cfg.Key,
			FallbackErrors: target.errors,
		}
		go func(key string) {
			results <- runResult{site: key, err: uiserver.Run(ctx, cfg)}
		}(target.cfg.Key)
	}

	var firstErr error
	var failingSite string
	for range siteTargets {
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

func normalizeSiteKey(siteArg string) string {
	return config.NormaliseSiteKey(siteArg)
}
