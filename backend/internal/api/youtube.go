package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Its-donkey/Sharpen-live/backend/internal/storage"
)

type youtubeURLParts struct {
	channelID string
	handle    string
	username  string
	custom    string
}

func (s *Server) enrichPlatforms(ctx context.Context, platforms []storage.Platform) []storage.Platform {
	if len(platforms) == 0 {
		return platforms
	}

	for i := range platforms {
		if !strings.EqualFold(platforms[i].Name, "youtube") {
			continue
		}

		parts := parseYouTubeChannelURL(platforms[i].ChannelURL)
		if parts.channelID != "" {
			platforms[i].ID = parts.channelID
			s.ensureYouTubeSubscription(ctx, platforms[i].ID)
			continue
		}

		if s.youtubeAPIKey == "" || s.httpClient == nil {
			continue
		}

		id, err := s.lookupYouTubeChannelID(ctx, parts)
		if err != nil || id == "" {
			continue
		}
		platforms[i].ID = id
		s.ensureYouTubeSubscription(ctx, id)
	}

	return platforms
}

func parseYouTubeChannelURL(raw string) youtubeURLParts {
	result := youtubeURLParts{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return result
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return result
	}

	host := strings.ToLower(parsed.Host)
	if !(strings.Contains(host, "youtube.com")) {
		return result
	}

	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return result
	}

	segments := strings.Split(path, "/")
	first := segments[0]

	switch {
	case first == "channel" && len(segments) > 1:
		result.channelID = segments[1]
	case strings.HasPrefix(first, "@"):
		result.handle = strings.TrimPrefix(first, "@")
	case first == "user" && len(segments) > 1:
		result.username = segments[1]
	case first == "c" && len(segments) > 1:
		result.custom = segments[1]
	default:
		// Support URLs like /@handle/videos
		if strings.HasPrefix(first, "@") {
			result.handle = strings.TrimPrefix(first, "@")
		}
	}

	return result
}

func (s *Server) lookupYouTubeChannelID(ctx context.Context, parts youtubeURLParts) (string, error) {
	if parts.handle != "" {
		params := url.Values{
			"part":      []string{"id"},
			"forHandle": []string{parts.handle},
			"key":       []string{s.youtubeAPIKey},
		}
		return s.fetchChannelsID(ctx, params)
	}

	if parts.username != "" {
		params := url.Values{
			"part":        []string{"id"},
			"forUsername": []string{parts.username},
			"key":         []string{s.youtubeAPIKey},
		}
		return s.fetchChannelsID(ctx, params)
	}

	if parts.custom != "" {
		return s.searchChannelID(ctx, parts.custom)
	}

	return "", nil
}

func (s *Server) fetchChannelsID(ctx context.Context, params url.Values) (string, error) {
	endpoint := url.URL{
		Scheme:   "https",
		Host:     "www.googleapis.com",
		Path:     "/youtube/v3/channels",
		RawQuery: params.Encode(),
	}
	return s.executeYouTubeRequest(ctx, endpoint.String(), decodeChannelsResponse)
}

func (s *Server) searchChannelID(ctx context.Context, query string) (string, error) {
	params := url.Values{
		"part":       []string{"snippet"},
		"type":       []string{"channel"},
		"q":          []string{query},
		"maxResults": []string{"1"},
		"key":        []string{s.youtubeAPIKey},
	}
	endpoint := url.URL{
		Scheme:   "https",
		Host:     "www.googleapis.com",
		Path:     "/youtube/v3/search",
		RawQuery: params.Encode(),
	}
	return s.executeYouTubeRequest(ctx, endpoint.String(), decodeSearchResponse)
}

func (s *Server) executeYouTubeRequest(ctx context.Context, endpoint string, decode func([]byte) (string, error)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("youtube api: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return decode(data)
}

func decodeChannelsResponse(data []byte) (string, error) {
	var payload struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if len(payload.Items) == 0 {
		return "", nil
	}
	return strings.TrimSpace(payload.Items[0].ID), nil
}

func decodeSearchResponse(data []byte) (string, error) {
	var payload struct {
		Items []struct {
			ID struct {
				ChannelID string `json:"channelId"`
			} `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	if len(payload.Items) == 0 {
		return "", nil
	}
	return strings.TrimSpace(payload.Items[0].ID.ChannelID), nil
}

func (s *Server) ensureYouTubeSubscription(ctx context.Context, channelID string) {
	if !s.youtubeAlerts.enabled || channelID == "" || s.httpClient == nil {
		return
	}

	if ctx == nil {
		ctx = context.Background()
	}

	params := url.Values{
		"hub.mode":     []string{"subscribe"},
		"hub.topic":    []string{fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", channelID)},
		"hub.callback": []string{s.youtubeAlerts.callbackURL},
		"hub.verify":   []string{"async"},
	}

	if token := s.youtubeVerifyToken(channelID); token != "" {
		params.Set("hub.verify_token", token)
	}

	if s.youtubeAlerts.secret != "" {
		params.Set("hub.secret", s.youtubeAlerts.secret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.youtubeHubURL, strings.NewReader(params.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, resp.Body)
		return
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	fmt.Printf("youtube subscription failed: %s: %s\n", resp.Status, strings.TrimSpace(string(body)))
}

func (s *Server) youtubeVerifyToken(channelID string) string {
	if channelID == "" {
		return ""
	}
	if s.youtubeAlerts.verifyPref == "" && s.youtubeAlerts.verifySuff == "" {
		return ""
	}
	return s.youtubeAlerts.verifyPref + channelID + s.youtubeAlerts.verifySuff
}
