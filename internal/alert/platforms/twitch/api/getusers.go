package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const usersEndpoint = "https://api.twitch.tv/helix/users"

// User represents a Twitch user record returned by the Helix users endpoint.
type User struct {
	ID              string `json:"id"`
	Login           string `json:"login"`
	DisplayName     string `json:"display_name"`
	Type            string `json:"type"`
	BroadcasterType string `json:"broadcaster_type"`
	Description     string `json:"description"`
	ProfileImageURL string `json:"profile_image_url"`
	OfflineImageURL string `json:"offline_image_url"`
	ViewCount       int64  `json:"view_count"`
	Email           string `json:"email,omitempty"`
	CreatedAt       string `json:"created_at"`
}

type usersResponse struct {
	Data []User `json:"data"`
}

// GetUsers fetches user records by ID and/or login. At least one id/login is required.
// The caller must supply an Authenticator configured with app or user credentials.
func GetUsers(ctx context.Context, client *http.Client, auth *Authenticator, ids []string, logins []string) ([]User, error) {
	if auth == nil {
		return nil, fmt.Errorf("twitch authenticator is required")
	}
	if client == nil {
		client = http.DefaultClient
	}

	idParams := dedupeParams(ids)
	loginParams := dedupeParams(logins)
	if len(idParams) == 0 && len(loginParams) == 0 {
		return nil, fmt.Errorf("at least one id or login is required")
	}
	if len(idParams) > 100 || len(loginParams) > 100 {
		return nil, fmt.Errorf("too many ids/logins: max 100 each")
	}

	q := url.Values{}
	for _, id := range idParams {
		q.Add("id", id)
	}
	for _, login := range loginParams {
		q.Add("login", login)
	}

	endpoint := usersEndpoint
	if enc := q.Encode(); enc != "" {
		endpoint += "?" + enc
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create users request: %w", err)
	}
	if err := auth.Apply(ctx, req); err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute users request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("users request failed: status %d", resp.StatusCode)
	}

	var body usersResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode users response: %w", err)
	}

	return body.Data, nil
}

func dedupeParams(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
