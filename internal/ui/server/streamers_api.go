package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

type streamersWatchOptions struct {
	FilePath     string
	PollInterval time.Duration
	SiteKey      string
}

const defaultWatchPollInterval = 2 * time.Second

func streamersWatchHandler(opts streamersWatchOptions) http.HandlerFunc {
	interval := opts.PollInterval
	if interval <= 0 {
		interval = defaultWatchPollInterval
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if opts.FilePath == "" {
			http.Error(w, "streamers path not configured", http.StatusInternalServerError)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		lastMod, _ := fileModTime(opts.FilePath)
		writeWatchMessage(w, flusher, lastMod)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				mod, err := fileModTime(opts.FilePath)
				if err != nil {
					continue
				}
				if mod.After(lastMod) {
					lastMod = mod
					writeWatchMessage(w, flusher, mod)
				}
			}
		}
	}
}

func (s *server) serveStreamersJSON(w http.ResponseWriter, r *http.Request) {
	if s.streamersStore == nil {
		http.Error(w, "streamers store unavailable", http.StatusInternalServerError)
		return
	}
	http.ServeFile(w, r, s.streamersStore.Path())
}

func writeWatchMessage(w http.ResponseWriter, flusher http.Flusher, ts time.Time) {
	fmt.Fprintf(w, "data: %d\n\n", ts.UnixMilli())
	flusher.Flush()
}

func mapStoreStreamerRecords(records []streamers.Record) []streamers.Streamer {
	out := make([]streamers.Streamer, 0, len(records))
	for _, r := range records {
		out = append(out, r.Streamer)
	}
	return out
}
