package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/alert/logging"
	"github.com/Its-donkey/Sharpen-live/internal/alert/platforms/youtube/ratelimit"
	"github.com/google/uuid"
)

// SearchLiveResult represents the live search result for a channel.
type SearchLiveResult struct {
	VideoID   string
	StartedAt time.Time
	ChannelID string
}

// SearchClient queries the YouTube Data API search endpoint.
type SearchClient struct {
	APIKey     string
	HTTPClient *http.Client
	Logger     logging.Logger
	BaseURL    string
}

// LiveNow returns the current live video (if any) for the channel.
func (c SearchClient) LiveNow(ctx context.Context, channelID string) (SearchLiveResult, error) {
	ch := strings.TrimSpace(channelID)
	if ch == "" {
		return SearchLiveResult{}, fmt.Errorf("channelID required")
	}

	requestID := logging.RequestIDFromContext(ctx)
	if strings.TrimSpace(requestID) == "" {
		requestID = strings.ToUpper(uuid.New().String())
	}

	apiKey := strings.TrimSpace(c.APIKey)
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	client = ratelimit.Client(client)

	endpoint, _ := url.Parse(c.baseURL())
	params := []string{
		"part=" + url.QueryEscape("snippet"),
		"channelId=" + url.QueryEscape(ch),
		"eventType=" + url.QueryEscape("live"),
		"type=" + url.QueryEscape("video"),
	}
	if apiKey != "" {
		params = append(params, "key="+url.QueryEscape(apiKey))
	}
	endpoint.RawQuery = strings.Join(params, "&")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return SearchLiveResult{}, err
	}

	logSearchRequest(c.Logger, requestID, req)
	start := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		logSearchFailure(c.Logger, requestID, req, err, time.Since(start))
		return SearchLiveResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return SearchLiveResult{}, fmt.Errorf("read search response: %w", err)
	}
	logSearchResponse(c.Logger, requestID, req, resp, body, time.Since(start))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return SearchLiveResult{}, fmt.Errorf("search API %s: %s", resp.Status, string(body))
	}

	var payload searchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return SearchLiveResult{}, fmt.Errorf("decode search response: %w", err)
	}

	for _, item := range payload.Items {
		id := strings.TrimSpace(item.ID.VideoID)
		if id == "" {
			continue
		}
		result := SearchLiveResult{
			VideoID:   id,
			ChannelID: strings.TrimSpace(item.Snippet.ChannelID),
		}
		if ts := strings.TrimSpace(item.Snippet.PublishedAt); ts != "" {
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				result.StartedAt = parsed
			}
		}
		return result, nil
	}

	return SearchLiveResult{}, nil
}

func (c SearchClient) baseURL() string {
	if trimmed := strings.TrimSpace(c.BaseURL); trimmed != "" {
		return trimmed
	}
	return "https://www.googleapis.com/youtube/v3/search"
}

type searchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			PublishedAt string `json:"publishedAt"`
			ChannelID   string `json:"channelId"`
		} `json:"snippet"`
	} `json:"items"`
}

func logSearchRequest(logger logging.Logger, id string, req *http.Request) {
	if logger == nil || req == nil {
		return
	}
	if dump, err := httputil.DumpRequestOut(req, true); err == nil {
		logging.LogWithID(logger, "http", id, fmt.Sprintf("YouTube search request\n%s", string(dump)))
	} else {
		logging.LogWithID(logger, "http", id, fmt.Sprintf("YouTube search request dump failed: %v", err))
	}
}

func logSearchResponse(logger logging.Logger, id string, req *http.Request, resp *http.Response, body []byte, dur time.Duration) {
	if logger == nil || resp == nil || req == nil {
		return
	}
	copyResp := *resp
	copyResp.Body = io.NopCloser(strings.NewReader(string(body)))
	if dump, err := httputil.DumpResponse(&copyResp, true); err == nil {
		logging.LogWithID(logger, "http", id, fmt.Sprintf("YouTube search response (%s %s) in %dms\n%s", req.Method, req.URL.String(), dur.Milliseconds(), string(dump)))
	} else {
		logging.LogWithID(logger, "http", id, fmt.Sprintf("YouTube search response dump failed for %s %s: %v", req.Method, req.URL.String(), err))
	}
}

func logSearchFailure(logger logging.Logger, id string, req *http.Request, err error, dur time.Duration) {
	if logger == nil || req == nil || err == nil {
		return
	}
	logging.LogWithID(logger, "http", id, fmt.Sprintf("YouTube search request failed after %dms (%s %s): %v", dur.Milliseconds(), req.Method, req.URL.String(), err))
}
