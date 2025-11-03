package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Its-donkey/Sharpen-live/backend/internal/server"
	"github.com/Its-donkey/Sharpen-live/backend/internal/storage"
)

const (
	adminToken    = "secret-token"
	adminEmail    = "admin@example.com"
	adminPassword = "strong-password"
)

type testEnv struct {
	store   *storage.JSONStore
	handler http.Handler
	server  *server.Server
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewJSONStore(
		filepath.Join(dir, "streamers.json"),
		filepath.Join(dir, "submissions.json"),
	)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	srv := server.New(store, adminToken, adminEmail, adminPassword)
	handler := srv.Handler(http.NotFoundHandler())
	return testEnv{store: store, handler: handler, server: srv}
}

func performRequest(handler http.Handler, method, target string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		buf = bytes.NewBuffer(data)
	} else {
		buf = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, target, buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestSubmitAndApproveFlow(t *testing.T) {
	env := newTestEnv(t)

	// Ensure login fails with incorrect credentials.
	invalidResp := performRequest(env.handler, http.MethodPost, "/api/admin/login", map[string]string{
		"email":    adminEmail,
		"password": "wrong",
	}, nil)
	if invalidResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from invalid login, got %d", invalidResp.Code)
	}

	// Ensure login succeeds and returns token.
	loginResp := performRequest(env.handler, http.MethodPost, "/api/admin/login", map[string]string{
		"email":    adminEmail,
		"password": adminPassword,
	}, nil)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("expected 200 from login, got %d", loginResp.Code)
	}

	submissionPayload := map[string]any{
		"name":        "EdgeCrafter",
		"description": "Sharpening demo",
		"status":      "online",
		"statusLabel": "Online",
		"languages":   []string{"English"},
		"platforms": []map[string]string{
			{"name": "Twitch", "channelUrl": "https://example.com", "liveUrl": "https://example.com/live"},
		},
	}

	resp := performRequest(env.handler, http.MethodPost, "/api/submit-streamer", submissionPayload, nil)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202 status, got %d", resp.Code)
	}

	submissions, err := env.store.ListSubmissions()
	if err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("expected 1 submission, got %d", len(submissions))
	}

	// Unauthorized admin request should fail.
	unauth := performRequest(env.handler, http.MethodGet, "/api/admin/submissions", nil, nil)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing admin token, got %d", unauth.Code)
	}

	headers := map[string]string{"Authorization": "Bearer " + adminToken}
	list := performRequest(env.handler, http.MethodGet, "/api/admin/submissions", nil, headers)
	if list.Code != http.StatusOK {
		t.Fatalf("expected 200 from admin submissions, got %d", list.Code)
	}

	approveBody := map[string]any{"action": "approve", "id": submissions[0].ID}
	approve := performRequest(env.handler, http.MethodPost, "/api/admin/submissions", approveBody, headers)
	if approve.Code != http.StatusOK {
		t.Fatalf("expected 200 approving submission, got %d", approve.Code)
	}

	streamersResp := performRequest(env.handler, http.MethodGet, "/api/streamers", nil, nil)
	if streamersResp.Code != http.StatusOK {
		t.Fatalf("expected 200 listing streamers, got %d", streamersResp.Code)
	}

	var streamers []storage.Streamer
	if err := json.Unmarshal(streamersResp.Body.Bytes(), &streamers); err != nil {
		t.Fatalf("decode streamers: %v", err)
	}
	if len(streamers) != 1 {
		t.Fatalf("expected 1 streamer after approval, got %d", len(streamers))
	}
	if streamers[0].Status != "online" {
		t.Fatalf("expected status online, got %s", streamers[0].Status)
	}
}

func TestRejectAndDeleteStreamers(t *testing.T) {
	env := newTestEnv(t)

	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	// Seed streamer directly
	created, err := env.store.CreateStreamer(storage.Streamer{
		Name:        "Existing",
		Description: "Sharpening",
		Status:      "offline",
		StatusLabel: "Offline",
		Languages:   []string{"English"},
		Platforms: []storage.Platform{
			{Name: "YouTube", ChannelURL: "https://example.com", LiveURL: "https://example.com/live"},
		},
	})
	if err != nil {
		t.Fatalf("seed streamer: %v", err)
	}

	// Rejecting a nonexistent submission should fail with 404.
	reject := performRequest(env.handler, http.MethodPost, "/api/admin/submissions", map[string]string{
		"action": "reject",
		"id":     "unknown",
	}, headers)
	if reject.Code != http.StatusNotFound {
		t.Fatalf("expected 404 rejecting missing submission, got %d", reject.Code)
	}

	// Update streamer via admin endpoint.
	update := performRequest(env.handler, http.MethodPut, "/api/admin/streamers/"+created.ID, map[string]any{
		"name":        "Existing",
		"description": "Updated",
		"status":      "busy",
		"statusLabel": "Workshop",
		"languages":   []string{"English", "German"},
		"platforms": []map[string]string{
			{"name": "YouTube", "channelUrl": "https://example.com", "liveUrl": "https://example.com/live"},
		},
	}, headers)
	if update.Code != http.StatusOK {
		t.Fatalf("expected 200 updating streamer, got %d", update.Code)
	}

	// Delete streamer
	del := performRequest(env.handler, http.MethodDelete, "/api/admin/streamers/"+created.ID, nil, headers)
	if del.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting streamer, got %d", del.Code)
	}
}

func TestAdminSettingsHandlers(t *testing.T) {
	env := newTestEnv(t)
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	t.Setenv("LISTEN_ADDR", ":9000")
	t.Setenv("SHARPEN_DATA_DIR", "/tmp/data")
	t.Setenv("SHARPEN_STATIC_DIR", "/tmp/static")
	t.Setenv("SHARPEN_STREAMERS_FILE", "/tmp/streamers.json")
	t.Setenv("SHARPEN_SUBMISSIONS_FILE", "/tmp/submissions.json")

	resp := performRequest(env.handler, http.MethodGet, "/api/admin/settings", nil, headers)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 from settings get, got %d", resp.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if payload["adminEmail"] != adminEmail {
		t.Fatalf("expected admin email %q, got %q", adminEmail, payload["adminEmail"])
	}

	newToken := "updated-token"
	updateResp := performRequest(env.handler, http.MethodPut, "/api/admin/settings", map[string]string{
		"adminToken": newToken,
	}, headers)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("expected 200 updating settings, got %d", updateResp.Code)
	}

	// verify token updated
	testHeaders := map[string]string{"Authorization": "Bearer " + newToken}
	okResp := performRequest(env.handler, http.MethodGet, "/api/admin/settings", nil, testHeaders)
	if okResp.Code != http.StatusOK {
		t.Fatalf("expected authorized with new token, got %d", okResp.Code)
	}
}
