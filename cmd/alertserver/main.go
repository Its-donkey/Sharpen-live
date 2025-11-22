package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Its-donkey/Sharpen-live/internal/alert/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, app.Options{}); err != nil {
		fmt.Fprintf(os.Stderr, "alertserver exited with error: %v\n", err)
		os.Exit(1)
	}
}
