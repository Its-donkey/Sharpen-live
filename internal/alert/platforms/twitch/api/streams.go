package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const streamsEndpoint = "https://api.twitch.tv/helix/streams"

// Stream represents a live stream from the Twitch Helix streams endpoint.
type Stream struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	UserLogin    string    `json:"user_login"`
	UserName     string    `json:"user_name"`
	GameID       string    `json:"game_id"`
	GameName     string    `json:"game_name"`
	Type         string    `json:"type"` // "live" or empty
	Title        string    `json:"title"`
	ViewerCount  int       `json:"viewer_count"`
	StartedAt    time.Time `json:"started_at"`
	Language     string    `json:"language"`
	ThumbnailURL string    `json:"thumbnail_url"`
	TagIDs       []string  `json:"tag_ids"`
	Tags         []string  `json:"tags"`
	IsMature     bool      `json:"is_mature"`
}

// StreamResult contains the result of a live status check for a broadcaster.
type StreamResult struct {
	IsLive      bool
	StreamID    string
	StartedAt   time.Time
	Title       string
	ViewerCount int
	GameName    string
}

type streamsResponse struct {
	Data       []Stream   `json:"data"`
	Pagination pagination `json:"pagination"`
}

type pagination struct {
	Cursor string `json:"cursor"`
}

// GetStreams fetches live stream information for the given broadcaster IDs.
// Returns a map of broadcaster ID -> StreamResult.
// If a broadcaster is not live, they will not appear in the response.
func GetStreams(ctx context.Context, client *http.Client, auth *Authenticator, broadcasterIDs []string) (map[string]StreamResult, error) {
	if auth == nil {
		return nil, fmt.Errorf("twitch authenticator is required")
	}
	if client == nil {
		client = http.DefaultClient
	}

	ids := dedupeParams(broadcasterIDs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one broadcaster ID is required")
	}
	if len(ids) > 100 {
		return nil, fmt.Errorf("too many broadcaster IDs: max 100")
	}

	q := url.Values{}
	for _, id := range ids {
		q.Add("user_id", id)
	}
	q.Set("first", "100")

	endpoint := streamsEndpoint + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create streams request: %w", err)
	}
	if err := auth.Apply(ctx, req); err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute streams request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("streams request failed: status %d", resp.StatusCode)
	}

	var body streamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode streams response: %w", err)
	}

	results := make(map[string]StreamResult)
	for _, stream := range body.Data {
		results[stream.UserID] = StreamResult{
			IsLive:      stream.Type == "live",
			StreamID:    stream.ID,
			StartedAt:   stream.StartedAt,
			Title:       stream.Title,
			ViewerCount: stream.ViewerCount,
			GameName:    stream.GameName,
		}
	}

	return results, nil
}

// IsLive checks if a single broadcaster is currently live.
// Returns the stream result if live, or a zero StreamResult if offline.
func IsLive(ctx context.Context, client *http.Client, auth *Authenticator, broadcasterID string) (StreamResult, error) {
	results, err := GetStreams(ctx, client, auth, []string{broadcasterID})
	if err != nil {
		return StreamResult{}, err
	}

	if result, ok := results[broadcasterID]; ok {
		return result, nil
	}

	// Not in results means offline
	return StreamResult{IsLive: false}, nil
}
