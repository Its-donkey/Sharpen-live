package logging

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileWriter writes logs to rotating files with compression.
type FileWriter struct {
	mu           sync.Mutex
	dir          string
	filename     string
	maxSize      int64
	maxFiles     int
	currentFile  *os.File
	currentSize  int64
	lastRotation time.Time
}

// NewFileWriter creates a new file writer with rotation.
func NewFileWriter(dir, filename string, maxSizeMB, maxFiles int) (*FileWriter, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	fw := &FileWriter{
		dir:          dir,
		filename:     filename,
		maxSize:      int64(maxSizeMB) * 1024 * 1024,
		maxFiles:     maxFiles,
		lastRotation: time.Now(),
	}

	if err := fw.openFile(); err != nil {
		return nil, err
	}

	return fw, nil
}

func (fw *FileWriter) openFile() error {
	path := filepath.Join(fw.dir, fw.filename)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("stat log file: %w", err)
	}

	fw.currentFile = f
	fw.currentSize = info.Size()
	return nil
}

func (fw *FileWriter) Write(p []byte) (n int, err error) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Check if rotation is needed
	if fw.shouldRotate(int64(len(p))) {
		if err := fw.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = fw.currentFile.Write(p)
	fw.currentSize += int64(n)
	return n, err
}

func (fw *FileWriter) shouldRotate(writeSize int64) bool {
	// Rotate if file will exceed max size
	if fw.currentSize+writeSize > fw.maxSize {
		return true
	}
	// Also rotate daily
	if time.Since(fw.lastRotation) > 24*time.Hour {
		return true
	}
	return false
}

func (fw *FileWriter) rotate() error {
	// Close current file
	if fw.currentFile != nil {
		if err := fw.currentFile.Close(); err != nil {
			return fmt.Errorf("close current file: %w", err)
		}
	}

	// Rename current file with timestamp
	oldPath := filepath.Join(fw.dir, fw.filename)
	timestamp := time.Now().Format("20060102-150405")
	newPath := filepath.Join(fw.dir, fmt.Sprintf("%s.%s", fw.filename, timestamp))

	if err := os.Rename(oldPath, newPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rename log file: %w", err)
	}

	// Compress the rotated file in background
	go fw.compressFile(newPath)

	// Clean up old files
	go fw.cleanup()

	// Open new file
	if err := fw.openFile(); err != nil {
		return err
	}

	fw.lastRotation = time.Now()
	return nil
}

func (fw *FileWriter) compressFile(path string) {
	gzPath := path + ".gz"
	in, err := os.Open(path)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(gzPath)
	if err != nil {
		return
	}
	defer out.Close()

	gzWriter := gzip.NewWriter(out)
	defer gzWriter.Close()

	if _, err := io.Copy(gzWriter, in); err != nil {
		os.Remove(gzPath)
		return
	}

	// Remove uncompressed file on success
	os.Remove(path)
}

func (fw *FileWriter) cleanup() {
	fw.mu.Lock()
	dir := fw.dir
	filename := fw.filename
	maxFiles := fw.maxFiles
	fw.mu.Unlock()

	// List all log files
	pattern := filepath.Join(dir, filename+".*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	// Sort by modification time (oldest first)
	sort.Slice(matches, func(i, j int) bool {
		infoI, errI := os.Stat(matches[i])
		infoJ, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	// Remove oldest files if exceeding limit
	if len(matches) > maxFiles {
		for _, path := range matches[:len(matches)-maxFiles] {
			os.Remove(path)
		}
	}
}

// Close closes the file writer.
func (fw *FileWriter) Close() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.currentFile != nil {
		return fw.currentFile.Close()
	}
	return nil
}

// ReadRecent reads the most recent N log entries from the current file.
func ReadRecent(logPath string, n int) ([]Entry, error) {
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Take last N lines
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}
	lines = lines[start:]

	// Parse entries
	entries := make([]Entry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip malformed entries
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
