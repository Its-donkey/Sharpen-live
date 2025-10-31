package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	pollInterval   = 5 * time.Minute
	requestTimeout = 15 * time.Second
)

// StreamAlert represents the payload sent when a channel goes live.
type StreamAlert struct {
	ChannelID string `json:"channelId"`
	StreamID  string `json:"streamId,omitempty"`
	Status    string `json:"status"`
}

// StreamChecker determines whether a stream is still active.
type StreamChecker interface {
	IsLive(ctx context.Context, channelID, streamID string) (bool, error)
}

// Monitor manages active live stream checks.
type Monitor struct {
	checker StreamChecker

	mu       sync.Mutex
	watchers map[string]context.CancelFunc
}

func NewMonitor(checker StreamChecker) *Monitor {
	return &Monitor{
		checker:  checker,
		watchers: make(map[string]context.CancelFunc),
	}
}

// HandleAlert registers a new alert and begins polling at the configured interval.
func (m *Monitor) HandleAlert(alert StreamAlert) {
	if alert.ChannelID == "" {
		log.Printf("ignoring alert with missing channel ID: %+v", alert)
		return
	}

	if alert.Status != "online" {
		m.cancelWatcher(alert.ChannelID)
		log.Printf("status for %s updated to %s; stopping monitor", alert.ChannelID, alert.Status)
		return
	}

	m.mu.Lock()
	if _, exists := m.watchers[alert.ChannelID]; exists {
		m.mu.Unlock()
		log.Printf("already monitoring channel %s", alert.ChannelID)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.watchers[alert.ChannelID] = cancel
	m.mu.Unlock()

	go m.watch(ctx, alert)
	log.Printf("began monitoring channel %s", alert.ChannelID)
}

func (m *Monitor) cancelWatcher(channelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, ok := m.watchers[channelID]; ok {
		cancel()
		delete(m.watchers, channelID)
	}
}

func (m *Monitor) watch(ctx context.Context, alert StreamAlert) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("monitor for %s cancelled", alert.ChannelID)
			return
		case <-ticker.C:
			live, err := m.checker.IsLive(ctx, alert.ChannelID, alert.StreamID)
			if err != nil {
				log.Printf("live check for %s failed: %v", alert.ChannelID, err)
				continue
			}
			if !live {
				log.Printf("channel %s is no longer live; stopping monitor", alert.ChannelID)
				m.cancelWatcher(alert.ChannelID)
				return
			}
			log.Printf("channel %s still live", alert.ChannelID)
		}
	}
}

// YouTubeChecker queries the YouTube Data API to determine if a stream is active.
type YouTubeChecker struct {
	APIKey     string
	HTTPClient *http.Client
}

var errMissingAPIKey = errors.New("missing YOUTUBE_API_KEY environment variable")

func (c *YouTubeChecker) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: requestTimeout}
}

func (c *YouTubeChecker) IsLive(ctx context.Context, channelID, streamID string) (bool, error) {
	if c.APIKey == "" {
		return false, errMissingAPIKey
	}

	if streamID != "" {
		return c.lookupStream(ctx, streamID)
	}

	return c.lookupChannel(ctx, channelID)
}

func (c *YouTubeChecker) lookupStream(ctx context.Context, streamID string) (bool, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://www.googleapis.com/youtube/v3/videos",
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("build videos request: %w", err)
	}

	q := req.URL.Query()
	q.Set("part", "liveStreamingDetails")
	q.Set("id", streamID)
	q.Set("key", c.APIKey)
	req.URL.RawQuery = q.Encode()

	resp, err := c.client().Do(req)
	if err != nil {
		return false, fmt.Errorf("execute videos request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("videos request failed: %s", resp.Status)
	}

	var payload struct {
		Items []struct {
			LiveStreamingDetails struct {
				ActualEndTime string `json:"actualEndTime"`
			} `json:"liveStreamingDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("decode videos response: %w", err)
	}

	if len(payload.Items) == 0 {
		return false, nil
	}

	return payload.Items[0].LiveStreamingDetails.ActualEndTime == "", nil
}

func (c *YouTubeChecker) lookupChannel(ctx context.Context, channelID string) (bool, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://www.googleapis.com/youtube/v3/search",
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("build search request: %w", err)
	}

	q := req.URL.Query()
	q.Set("part", "id")
	q.Set("channelId", channelID)
	q.Set("eventType", "live")
	q.Set("type", "video")
	q.Set("maxResults", "1")
	q.Set("key", c.APIKey)
	req.URL.RawQuery = q.Encode()

	resp, err := c.client().Do(req)
	if err != nil {
		return false, fmt.Errorf("execute search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("search request failed: %s", resp.Status)
	}

	var payload struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false, fmt.Errorf("decode search response: %w", err)
	}

	return len(payload.Items) > 0, nil
}

func main() {
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	checker := &YouTubeChecker{APIKey: apiKey}
	monitor := NewMonitor(checker)

	http.HandleFunc("/alerts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var alert StreamAlert
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		monitor.HandleAlert(alert)
		w.WriteHeader(http.StatusAccepted)
	})

	addr := ":8080"
	log.Printf("Sharpen Live YouTube alert listener running on %s", addr)
	if apiKey == "" {
		log.Println("warning: YOUTUBE_API_KEY not provided; live checks will fail until configured")
	}

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
