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

	listen := flag.String("listen", "", "address to serve the Sharpen.Live UI (defaults to config.json server.addr+port)")
	templatesDir := flag.String("templates", "", "path to the html/template files (defaults to config.json ui.templates)")
	assetsDir := flag.String("assets", "", "path where styles.css is located (defaults to config.json app.assets)")
	logDir := flag.String("logs", "", "directory for category logs (defaults to config.json app.logs)")
	dataDir := flag.String("data", "", "directory for data files (streamers/submissions); defaults to config.json app.data")
	configPath := flag.String("config", "config.json", "path to server configuration")
	flag.Parse()

	cfg := uiserver.Options{
		Listen:       *listen,
		TemplatesDir: *templatesDir,
		AssetsDir:    *assetsDir,
		LogDir:       *logDir,
		DataDir:      *dataDir,
		ConfigPath:   *configPath,
	}

	if err := uiserver.Run(ctx, cfg); err != nil && err != context.Canceled {
		log.Fatalf("server error: %v", err)
	}
}
