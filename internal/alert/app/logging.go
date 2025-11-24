package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/alert/logging"
)

type logWriterHolder struct {
	mu     sync.Mutex
	writer io.WriteCloser
}

func (h *logWriterHolder) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.writer == nil {
		return nil
	}
	err := h.writer.Close()
	h.writer = nil
	return err
}

func (h *logWriterHolder) Replace(next io.WriteCloser) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.writer != nil {
		_ = h.writer.Close()
	}
	h.writer = next
}

func configureLogging(logPath string) (*logWriterHolder, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	logWriter := logging.NewLogFileWriter(file)
	logging.SetDefaultWriter(io.MultiWriter(os.Stdout, logWriter))
	return &logWriterHolder{writer: logWriter}, nil
}

func startLogRotation(ctx context.Context, logPath string, holder *logWriterHolder, interval time.Duration) {
	if holder == nil {
		return
	}
	go func() {
		timer := time.NewTimer(interval)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		if err := rotateExistingLog(logPath, time.Now().UTC()); err != nil {
			fmt.Fprintf(os.Stderr, "rotate log file: %v\n", err)
			return
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open rotated log file: %v\n", err)
			return
		}
		next := logging.NewLogFileWriter(file)
		logging.SetDefaultWriter(io.MultiWriter(os.Stdout, next))
		holder.Replace(next)
	}()
}

func rotateExistingLog(logPath string, started time.Time) error {
	info, err := os.Stat(logPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat log file: %w", err)
	}
	if info.Size() == 0 {
		return nil
	}

	logDir := filepath.Dir(logPath)
	archiveDir := filepath.Join(logDir, "logs")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("create log archive dir: %w", err)
	}

	baseTimestamp := started.Format("2006-01-02_15-04-05")
	baseName := fmt.Sprintf("alertserver-%s.log", baseTimestamp)
	destPath := filepath.Join(archiveDir, baseName)
	for i := 1; ; i++ {
		if _, err := os.Stat(destPath); errors.Is(err, os.ErrNotExist) {
			break
		}
		destPath = filepath.Join(archiveDir, fmt.Sprintf("alertserver-%s-%d.log", baseTimestamp, i))
	}
	if err := os.Rename(logPath, destPath); err != nil {
		return fmt.Errorf("archive log file: %w", err)
	}
	return nil
}
