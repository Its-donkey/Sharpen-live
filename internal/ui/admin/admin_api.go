package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Its-donkey/Sharpen-live/internal/ui/model"
	"github.com/Its-donkey/Sharpen-live/internal/ui/state"
	streamersvc "github.com/Its-donkey/Sharpen-live/internal/ui/streamers"
)

func adminAPIRequest(ctx context.Context, method, path string, payload any, requireAuth bool) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, 0, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if requireAuth {
		token := strings.TrimSpace(state.AdminConsole.Token)
		if token == "" {
			return nil, 0, errors.New("admin token missing")
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if requireAuth && resp.StatusCode == http.StatusUnauthorized {
		handleAdminUnauthorizedResponse()
	}
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = resp.Status
		}
		return nil, resp.StatusCode, errors.New(message)
	}
	return responseBody, resp.StatusCode, nil
}

func adminLogin(ctx context.Context, email, password string) (string, error) {
	body, _, err := adminAPIRequest(ctx, http.MethodPost, "/api/admin/login", map[string]string{
		"email":    email,
		"password": password,
	}, false)
	if err != nil {
		return "", err
	}
	var resp model.LoginResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.Token) == "" {
		return "", errors.New("missing admin token")
	}
	return resp.Token, nil
}

func adminFetchSubmissions(ctx context.Context) ([]model.AdminSubmission, error) {
	body, _, err := adminAPIRequest(ctx, http.MethodGet, "/api/admin/submissions", nil, true)
	if err != nil {
		return nil, err
	}
	var submissions []model.AdminSubmission
	if err := json.Unmarshal(body, &submissions); err == nil && submissions != nil {
		return submissions, nil
	}
	var wrapped struct {
		Submissions []model.AdminSubmission `json:"submissions"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Submissions != nil {
		return wrapped.Submissions, nil
	}
	return nil, errors.New("unexpected submissions response")
}

func adminFetchStreamers(ctx context.Context) ([]model.Streamer, bool, error) {
	streamers, err := streamersvc.FetchStreamers(ctx)
	if err != nil && len(streamers) > 0 {
		return streamers, true, nil
	}
	return streamers, false, err
}

func adminUpdateStreamer(ctx context.Context, id string, payload model.AdminSubmissionPayload) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("streamer id is required")
	}
	type patchBody struct {
		Streamer struct {
			ID          string    `json:"id"`
			Alias       *string   `json:"alias,omitempty"`
			Description *string   `json:"description,omitempty"`
			Languages   *[]string `json:"languages,omitempty"`
		} `json:"streamer"`
	}

	body := patchBody{}
	body.Streamer.ID = id

	alias := strings.TrimSpace(payload.Name)
	if alias != "" {
		body.Streamer.Alias = &alias
	}
	desc := strings.TrimSpace(payload.Description)
	body.Streamer.Description = &desc
	langs := append([]string(nil), payload.Languages...)
	body.Streamer.Languages = &langs

	_, _, err := adminAPIRequest(ctx, http.MethodPatch, "/api/streamers", body, true)
	return err
}

func adminDeleteStreamer(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("streamer id is required")
	}
	body := map[string]any{
		"streamer": map[string]string{
			"id": id,
		},
	}
	_, _, err := adminAPIRequest(ctx, http.MethodDelete, "/api/streamers", body, true)
	return err
}

func adminModerateSubmission(ctx context.Context, action, id string) error {
	body := map[string]string{
		"action": action,
		"id":     id,
	}
	_, _, err := adminAPIRequest(ctx, http.MethodPost, "/api/admin/submissions", body, true)
	return err
}

func adminFetchSettings(ctx context.Context) (*model.AdminSettings, error) {
	body, status, err := adminAPIRequest(ctx, http.MethodGet, "/api/admin/settings", nil, true)
	if err != nil {
		if status == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	var settings model.AdminSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func adminUpdateSettings(ctx context.Context, updates model.AdminSettingsUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	_, _, err := adminAPIRequest(ctx, http.MethodPut, "/api/admin/settings", updates, true)
	return err
}

func adminFetchMonitor(ctx context.Context) ([]model.AdminMonitorEvent, map[string]model.YouTubeLeaseStatus, error) {
	body, status, err := adminAPIRequest(ctx, http.MethodGet, "/api/admin/monitor/youtube", nil, true)
	if err != nil {
		if status == http.StatusNotFound {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var resp adminMonitorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, err
	}
	events, leases := convertMonitorResponse(resp)
	return events, leases, nil
}

type adminMonitorResponse struct {
	Summary adminMonitorSummary  `json:"summary"`
	Records []adminMonitorRecord `json:"records"`
}

type adminMonitorSummary struct {
	Total    int `json:"total"`
	Healthy  int `json:"healthy"`
	Renewing int `json:"renewing"`
	Expired  int `json:"expired"`
	Pending  int `json:"pending"`
}

type adminMonitorRecord struct {
	StreamerID         string `json:"streamerId"`
	Alias              string `json:"alias"`
	ChannelID          string `json:"channelId"`
	Handle             string `json:"handle"`
	HubURL             string `json:"hubUrl"`
	CallbackURL        string `json:"callbackUrl"`
	LeaseSeconds       int    `json:"leaseSeconds"`
	LeaseStart         string `json:"leaseStart"`
	LeaseExpires       string `json:"leaseExpires"`
	RenewAt            string `json:"renewAt"`
	RenewWindowSeconds int    `json:"renewWindowSeconds"`
	Status             string `json:"status"`
}

const leaseExpiringSoonWindow = 12 * time.Hour

func convertMonitorResponse(resp adminMonitorResponse) ([]model.AdminMonitorEvent, map[string]model.YouTubeLeaseStatus) {
	if len(resp.Records) == 0 && resp.Summary.Total == 0 {
		return nil, nil
	}
	events := make([]model.AdminMonitorEvent, 0, len(resp.Records)+1)
	leases := make(map[string]model.YouTubeLeaseStatus)
	if resp.Summary.Total > 0 {
		message := fmt.Sprintf(
			"Total %d · healthy %d · renewing %d · expired %d · pending %d",
			resp.Summary.Total,
			resp.Summary.Healthy,
			resp.Summary.Renewing,
			resp.Summary.Expired,
			resp.Summary.Pending,
		)
		events = append(events, model.AdminMonitorEvent{
			Platform: "summary",
			Message:  message,
		})
	}
	for _, record := range resp.Records {
		message := fmt.Sprintf("%s (%s) status %s · Lease expires %s · Renew at %s · Callback %s",
			strings.TrimSpace(record.Alias),
			strings.TrimSpace(record.Handle),
			strings.ToUpper(strings.TrimSpace(record.Status)),
			strings.TrimSpace(record.LeaseExpires),
			strings.TrimSpace(record.RenewAt),
			strings.TrimSpace(record.CallbackURL),
		)
		events = append(events, model.AdminMonitorEvent{
			ID:        0,
			Platform:  "youtube",
			Timestamp: strings.TrimSpace(record.LeaseStart),
			Message:   message,
		})
		lease := buildLeaseStatus(record)
		for _, key := range leaseKeys(record) {
			if key == "" {
				continue
			}
			leases[key] = lease
		}
	}
	if len(leases) == 0 {
		return events, nil
	}
	return events, leases
}

func buildLeaseStatus(record adminMonitorRecord) model.YouTubeLeaseStatus {
	now := time.Now().UTC()
	expiryTime, _ := time.Parse(time.RFC3339, strings.TrimSpace(record.LeaseExpires))
	expired := !expiryTime.IsZero() && now.After(expiryTime)
	expiringSoon := !expired && !expiryTime.IsZero() && expiryTime.Sub(now) <= leaseExpiringSoonWindow

	status := strings.ToLower(strings.TrimSpace(record.Status))
	if status == "" {
		switch {
		case expired:
			status = "expired"
		case expiringSoon:
			status = "expiring"
		default:
			status = "valid"
		}
	}

	return model.YouTubeLeaseStatus{
		Alias:        strings.TrimSpace(record.Alias),
		Handle:       strings.TrimSpace(record.Handle),
		ChannelID:    strings.TrimSpace(record.ChannelID),
		LeaseStart:   strings.TrimSpace(record.LeaseStart),
		LeaseExpires: strings.TrimSpace(record.LeaseExpires),
		Status:       status,
		Expired:      expired,
		ExpiringSoon: expiringSoon,
		StartDate:    formatLeaseDate(record.LeaseStart),
	}
}

func formatLeaseDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format("2006-01-02")
	}
	if len(raw) >= 10 {
		return raw[:10]
	}
	return raw
}

func leaseKeys(record adminMonitorRecord) []string {
	keys := []string{
		normalizeLeaseKey(record.Alias),
		normalizeLeaseKey(record.Handle),
		normalizeLeaseKey(record.ChannelID),
	}
	return keys
}

func normalizeLeaseKey(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.TrimPrefix(raw, "@")
	return raw
}

func adminCheckChannelStatus(ctx context.Context) (model.AdminStatusCheckResult, error) {
	body, _, err := adminAPIRequest(ctx, http.MethodPost, "/api/admin/streamers/status", nil, true)
	if err != nil {
		return model.AdminStatusCheckResult{}, err
	}
	var result model.AdminStatusCheckResult
	if err := json.Unmarshal(body, &result); err != nil {
		return model.AdminStatusCheckResult{}, err
	}
	return result, nil
}
