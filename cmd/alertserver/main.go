package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	listen := flag.String("listen", "127.0.0.1:4173", "address to serve the Sharpen.Live UI")
	templatesDir := flag.String("templates", "ui/templates", "path to the html/template files")
	assetsDir := flag.String("assets", "ui", "path where styles.css is located")
	configPath := flag.String("config", "config.json", "path to server configuration")
	flag.Parse()

	cfg := uiserver.Options{
		Listen:       *listen,
		TemplatesDir: *templatesDir,
		AssetsDir:    *assetsDir,
		ConfigPath:   *configPath,
	}

	if err := uiserver.Run(ctx, cfg); err != nil && err != context.Canceled {
		log.Fatalf("server error: %v", err)
	}
}
