package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type procConfig struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	procs := []procConfig{
		{
			Name: "build-ui-wasm",
			Args: []string{"go", "build", "-o", "ui/main.wasm", "./cmd/ui-wasm"},
			Env:  []string{"GOOS=js", "GOARCH=wasm"},
		},
		{
			Name: "alertserver",
			Args: []string{"go", "run", "./cmd/alertserver"},
		},
		{
			Name: "ui",
			Args: []string{
				"go", "run", "./cmd/ui-server",
				"-listen", "127.0.0.1:4173",
				"-api", "http://127.0.0.1:8880",
				"-assets", "ui",
				"-templates", "ui/templates",
			},
		},
	}

	if err := runAll(ctx, procs); err != nil {
		fmt.Fprintf(os.Stderr, "sharpen-live exited with error: %v\n", err)
		os.Exit(1)
	}
}

func runAll(ctx context.Context, procs []procConfig) error {
	if len(procs) == 0 {
		return fmt.Errorf("no processes configured")
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(procs))

	for _, cfg := range procs {
		wg.Add(1)
		go func(cfg procConfig) {
			defer wg.Done()
			cmd := exec.CommandContext(ctx, cfg.Args[0], cfg.Args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if cfg.Dir != "" {
				cmd.Dir = cfg.Dir
			}
			if len(cfg.Env) > 0 {
				cmd.Env = append(append([]string{}, os.Environ()...), cfg.Env...)
			}
			if err := cmd.Start(); err != nil {
				errCh <- fmt.Errorf("%s start: %w", cfg.Name, err)
				return
			}
			if err := cmd.Wait(); err != nil {
				// If the context was cancelled, treat the exit as expected.
				select {
				case <-ctx.Done():
					return
				default:
				}
				errCh <- fmt.Errorf("%s exited: %w", cfg.Name, err)
			}
		}(cfg)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		shutdownDelay := time.After(2 * time.Second)
		select {
		case <-done:
		case <-shutdownDelay:
		}
	case err := <-errCh:
		return err
	case <-done:
	}
	return nil
}
