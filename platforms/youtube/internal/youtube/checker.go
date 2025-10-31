package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	videosEndpoint = "https://www.googleapis.com/youtube/v3/videos"
	searchEndpoint = "https://www.googleapis.com/youtube/v3/search"
	defaultTimeout = 15 * time.Second
)

// ErrMissingAPIKey is returned when the checker is invoked without configuration.
var ErrMissingAPIKey = errors.New("youtube: missing API key")

// Option provides functional configuration for the Checker.
type Option func(*Checker)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Checker) {
		c.httpClient = client
	}
}

// WithEndpoints overrides the default YouTube API endpoints. Useful for testing.
func WithEndpoints(videosURL, searchURL string) Option {
	return func(c *Checker) {
		c.videosURL = videosURL
		c.searchURL = searchURL
	}
}

// Checker queries the YouTube Data API to determine whether a channel or stream is live.
type Checker struct {
	apiKey     string
	httpClient *http.Client
	videosURL  string
	searchURL  string
}

// NewChecker returns a configured YouTube live status checker.
func NewChecker(apiKey string, opts ...Option) *Checker {
	c := &Checker{
		apiKey:    apiKey,
		videosURL: videosEndpoint,
		searchURL: searchEndpoint,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: defaultTimeout}
	}

	return c
}

// IsLive verifies whether the supplied channel or stream is currently live.
func (c *Checker) IsLive(ctx context.Context, channelID, streamID string) (bool, error) {
	if c.apiKey == "" {
		return false, ErrMissingAPIKey
	}

	if streamID != "" {
		return c.lookupStream(ctx, streamID)
	}

	return c.lookupChannel(ctx, channelID)
}

func (c *Checker) lookupStream(ctx context.Context, streamID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.videosURL, nil)
	if err != nil {
		return false, fmt.Errorf("build videos request: %w", err)
	}

	q := req.URL.Query()
	q.Set("part", "liveStreamingDetails")
	q.Set("id", streamID)
	q.Set("key", c.apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
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

func (c *Checker) lookupChannel(ctx context.Context, channelID string) (bool, error) {
	if channelID == "" {
		return false, errors.New("youtube: channel ID is required when stream ID missing")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.searchURL, nil)
	if err != nil {
		return false, fmt.Errorf("build search request: %w", err)
	}

	q := req.URL.Query()
	q.Set("part", "id")
	q.Set("channelId", channelID)
	q.Set("eventType", "live")
	q.Set("type", "video")
	q.Set("maxResults", "1")
	q.Set("key", c.apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := c.httpClient.Do(req)
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
