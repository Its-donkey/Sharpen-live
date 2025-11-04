package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Its-donkey/Sharpen-live/backend/internal/api"
	"github.com/Its-donkey/Sharpen-live/backend/internal/settings"
	"github.com/Its-donkey/Sharpen-live/backend/internal/storage"
)

const (
	adminToken    = "secret-token"
	adminEmail    = "admin@example.com"
	adminPassword = "strong-password"
)

const defaultHubURL = "https://pubsubhubbub.appspot.com/subscribe"

type testEnv struct {
	store   *storage.JSONStore
	handler http.Handler
	server  *api.Server
}

func newTestEnv(t *testing.T, initial *settings.Settings, opts ...api.Option) testEnv {
	t.Helper()
	dir := t.TempDir()
	streamersPath := filepath.Join(dir, "streamers.json")
	submissionsPath := filepath.Join(dir, "submissions.json")
	store, err := storage.NewJSONStore(streamersPath, submissionsPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	seed := settings.Settings{
		AdminToken:      adminToken,
		AdminEmail:      adminEmail,
		AdminPassword:   adminPassword,
		ListenAddr:      ":8880",
		DataDir:         filepath.Dir(streamersPath),
		StaticDir:       "frontend/dist",
		StreamersFile:   streamersPath,
		SubmissionsFile: submissionsPath,
	}
	if initial != nil {
		seed = mergeSettings(seed, *initial)
	}

	settingsStore := settings.NewMemoryStore(seed, true)
	srv := api.New(store, settingsStore, seed, opts...)
	handler := srv.Handler(http.NotFoundHandler())
	return testEnv{store: store, handler: handler, server: srv}
}

func mergeSettings(base, override settings.Settings) settings.Settings {
	if override.AdminToken != "" {
		base.AdminToken = override.AdminToken
	}
	if override.AdminEmail != "" {
		base.AdminEmail = override.AdminEmail
	}
	if override.AdminPassword != "" {
		base.AdminPassword = override.AdminPassword
	}
	if override.YouTubeAPIKey != "" {
		base.YouTubeAPIKey = override.YouTubeAPIKey
	}
	if override.YouTubeAlertsCallback != "" {
		base.YouTubeAlertsCallback = override.YouTubeAlertsCallback
	}
	if override.YouTubeAlertsSecret != "" {
		base.YouTubeAlertsSecret = override.YouTubeAlertsSecret
	}
	if override.YouTubeAlertsVerifyPrefix != "" {
		base.YouTubeAlertsVerifyPrefix = override.YouTubeAlertsVerifyPrefix
	}
	if override.YouTubeAlertsVerifySuffix != "" {
		base.YouTubeAlertsVerifySuffix = override.YouTubeAlertsVerifySuffix
	}
	if override.YouTubeAlertsHubURL != "" {
		base.YouTubeAlertsHubURL = override.YouTubeAlertsHubURL
	}
	if override.ListenAddr != "" {
		base.ListenAddr = override.ListenAddr
	}
	if override.DataDir != "" {
		base.DataDir = override.DataDir
	}
	if override.StaticDir != "" {
		base.StaticDir = override.StaticDir
	}
	if override.StreamersFile != "" {
		base.StreamersFile = override.StreamersFile
	}
	if override.SubmissionsFile != "" {
		base.SubmissionsFile = override.SubmissionsFile
	}
	return base
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
	env := newTestEnv(t, nil)

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
			{"name": "Twitch", "channelUrl": "https://example.com"},
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
	env := newTestEnv(t, nil)

	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	// Seed streamer directly
	created, err := env.store.CreateStreamer(storage.Streamer{
		Name:        "Existing",
		Description: "Sharpening",
		Status:      "offline",
		StatusLabel: "Offline",
		Languages:   []string{"English"},
		Platforms: []storage.Platform{
			{Name: "YouTube", ChannelURL: "https://example.com"},
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
			{"name": "YouTube", "channelUrl": "https://example.com"},
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

func TestDeleteStreamerUnsubscribesYouTube(t *testing.T) {
	var (
		mu      sync.Mutex
		modes   []string
		topics  []string
		tokens  []string
		secrets []string
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		values, err := url.ParseQuery(string(data))
		if err != nil {
			t.Fatalf("parse body: %v", err)
		}
		mu.Lock()
		modes = append(modes, values.Get("hub.mode"))
		topics = append(topics, values.Get("hub.topic"))
		tokens = append(tokens, values.Get("hub.verify_token"))
		secrets = append(secrets, values.Get("hub.secret"))
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	env := newTestEnv(t,
		api.WithYouTubeAlerts(api.YouTubeAlertsConfig{
			HubURL:            ts.URL,
			CallbackURL:       "https://alerts.sharpen.live/callback",
			Secret:            "secret-456",
			VerifyTokenPrefix: "prefix-",
			VerifyTokenSuffix: "-suffix",
		}),
		api.WithHTTPClient(ts.Client()),
	)

	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	created, err := env.store.CreateStreamer(storage.Streamer{
		Name:        "YouTuber",
		Description: "Streaming",
		Status:      "online",
		StatusLabel: "Online",
		Languages:   []string{"English"},
		Platforms: []storage.Platform{
			{
				Name:       "YouTube",
				ChannelURL: "https://www.youtube.com/channel/UC999",
				ID:         "UC999",
			},
		},
	})
	if err != nil {
		t.Fatalf("seed streamer: %v", err)
	}

	resp := performRequest(env.handler, http.MethodDelete, "/api/admin/streamers/"+created.ID, nil, headers)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting streamer, got %d", resp.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(modes) != 1 {
		t.Fatalf("expected 1 request, got %d", len(modes))
	}
	if modes[0] != "unsubscribe" {
		t.Fatalf("expected unsubscribe mode, got %s", modes[0])
	}
	if topics[0] != "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UC999" {
		t.Fatalf("unexpected topic: %s", topics[0])
	}
	if tokens[0] != "prefix-UC999-suffix" {
		t.Fatalf("unexpected verify token: %s", tokens[0])
	}
	if secrets[0] != "secret-456" {
		t.Fatalf("unexpected secret: %s", secrets[0])
	}
}

func TestAdminSettingsHandlers(t *testing.T) {
	initial := settings.Settings{
		AdminToken:      adminToken,
		AdminEmail:      adminEmail,
		AdminPassword:   adminPassword,
		ListenAddr:      ":9000",
		DataDir:         "/tmp/data",
		StaticDir:       "/tmp/static",
		StreamersFile:   "/tmp/streamers.json",
		SubmissionsFile: "/tmp/submissions.json",
	}

	env := newTestEnv(t, &initial)
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

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
	if payload["youtubeApiKey"] != "" {
		t.Fatalf("expected blank youtube key, got %q", payload["youtubeApiKey"])
	}
	if payload["youtubeAlertsCallback"] != "" {
		t.Fatalf("expected blank youtube alerts callback, got %q", payload["youtubeAlertsCallback"])
	}
	if payload["youtubeAlertsSecret"] != "" {
		t.Fatalf("expected blank youtube alerts secret, got %q", payload["youtubeAlertsSecret"])
	}
	if payload["youtubeAlertsVerifyPrefix"] != "" {
		t.Fatalf("expected blank youtube alerts verify prefix, got %q", payload["youtubeAlertsVerifyPrefix"])
	}
	if payload["youtubeAlertsVerifySuffix"] != "" {
		t.Fatalf("expected blank youtube alerts verify suffix, got %q", payload["youtubeAlertsVerifySuffix"])
	}
	if payload["youtubeAlertsHubUrl"] != defaultHubURL {
		t.Fatalf("expected default hub url %q, got %q", defaultHubURL, payload["youtubeAlertsHubUrl"])
	}

	newToken := "updated-token"
	newYouTubeKey := "api-key-123"
	newCallback := "https://sharpen.live/alerts"
	newSecret := "secret-123"
	newPrefix := "sharpen-"
	newSuffix := "-testing"
	customHub := "https://pubsubhubbub.example.com/subscribe"
	updateResp := performRequest(env.handler, http.MethodPut, "/api/admin/settings", map[string]string{
		"adminToken":                newToken,
		"youtubeApiKey":             newYouTubeKey,
		"youtubeAlertsCallback":     newCallback,
		"youtubeAlertsSecret":       newSecret,
		"youtubeAlertsVerifyPrefix": newPrefix,
		"youtubeAlertsVerifySuffix": newSuffix,
		"youtubeAlertsHubUrl":       customHub,
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

	var updated map[string]string
	if err := json.Unmarshal(okResp.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal updated settings: %v", err)
	}
	if updated["youtubeApiKey"] != newYouTubeKey {
		t.Fatalf("expected youtube key %q, got %q", newYouTubeKey, updated["youtubeApiKey"])
	}
	if updated["youtubeAlertsCallback"] != newCallback {
		t.Fatalf("expected alerts callback %q, got %q", newCallback, updated["youtubeAlertsCallback"])
	}
	if updated["youtubeAlertsSecret"] != newSecret {
		t.Fatalf("expected alerts secret %q, got %q", newSecret, updated["youtubeAlertsSecret"])
	}
	if updated["youtubeAlertsVerifyPrefix"] != newPrefix {
		t.Fatalf("expected alerts verify prefix %q, got %q", newPrefix, updated["youtubeAlertsVerifyPrefix"])
	}
	if updated["youtubeAlertsVerifySuffix"] != newSuffix {
		t.Fatalf("expected alerts verify suffix %q, got %q", newSuffix, updated["youtubeAlertsVerifySuffix"])
	}
	if updated["youtubeAlertsHubUrl"] != customHub {
		t.Fatalf("expected hub url %q, got %q", customHub, updated["youtubeAlertsHubUrl"])
	}

	disableResp := performRequest(env.handler, http.MethodPut, "/api/admin/settings", map[string]string{
		"youtubeAlertsCallback": "",
	}, testHeaders)
	if disableResp.Code != http.StatusOK {
		t.Fatalf("expected 200 disabling alerts, got %d", disableResp.Code)
	}

	disabled := performRequest(env.handler, http.MethodGet, "/api/admin/settings", nil, testHeaders)
	if disabled.Code != http.StatusOK {
		t.Fatalf("expected 200 fetching disabled settings, got %d", disabled.Code)
	}
	var disabledPayload map[string]string
	if err := json.Unmarshal(disabled.Body.Bytes(), &disabledPayload); err != nil {
		t.Fatalf("unmarshal disabled settings: %v", err)
	}
	if disabledPayload["youtubeAlertsCallback"] != "" {
		t.Fatalf("expected blank callback after disable, got %q", disabledPayload["youtubeAlertsCallback"])
	}
	if disabledPayload["youtubeAlertsSecret"] != "" {
		t.Fatalf("expected secret cleared after disable, got %q", disabledPayload["youtubeAlertsSecret"])
	}
	if disabledPayload["youtubeAlertsVerifyPrefix"] != "" {
		t.Fatalf("expected prefix cleared after disable, got %q", disabledPayload["youtubeAlertsVerifyPrefix"])
	}
	if disabledPayload["youtubeAlertsVerifySuffix"] != "" {
		t.Fatalf("expected suffix cleared after disable, got %q", disabledPayload["youtubeAlertsVerifySuffix"])
	}
	if disabledPayload["youtubeAlertsHubUrl"] != defaultHubURL {
		t.Fatalf("expected hub url reset to default %q, got %q", defaultHubURL, disabledPayload["youtubeAlertsHubUrl"])
	}
	if value := os.Getenv("YOUTUBE_ALERTS_HUB_URL"); value != "" {
		t.Fatalf("expected hub env cleared, got %q", value)
	}
}

func TestAdminYouTubeMonitor(t *testing.T) {
	var (
		mu    sync.Mutex
		calls []url.Values
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		_ = r.Body.Close()
		values, err := url.ParseQuery(string(data))
		if err != nil {
			t.Fatalf("parse body: %v", err)
		}
		mu.Lock()
		calls = append(calls, values)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	initial := settings.Settings{
		AdminToken:                adminToken,
		AdminEmail:                adminEmail,
		AdminPassword:             adminPassword,
		ListenAddr:                ":8880",
		DataDir:                   "/tmp/data",
		StaticDir:                 "frontend/dist",
		StreamersFile:             "/tmp/streamers.json",
		SubmissionsFile:           "/tmp/submissions.json",
		YouTubeAlertsCallback:     "https://alerts.sharpen.live/callback",
		YouTubeAlertsSecret:       "secret-789",
		YouTubeAlertsVerifyPrefix: "prefix-",
		YouTubeAlertsVerifySuffix: "-suffix",
		YouTubeAlertsHubURL:       ts.URL,
	}

	env := newTestEnv(t, &initial, api.WithHTTPClient(ts.Client()))
	headers := map[string]string{"Authorization": "Bearer " + adminToken}

	createResp := performRequest(env.handler, http.MethodPost, "/api/admin/streamers", map[string]any{
		"name":        "Monitor",
		"description": "Testing",
		"status":      "online",
		"statusLabel": "Online",
		"languages":   []string{"English"},
		"platforms": []map[string]string{
			{"name": "YouTube", "channelUrl": "https://www.youtube.com/channel/UC777"},
		},
	}, headers)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating streamer, got %d", createResp.Code)
	}

	var created storage.Streamer
	if err := json.Unmarshal(createResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created streamer: %v", err)
	}

	resp := performRequest(env.handler, http.MethodDelete, "/api/admin/streamers/"+created.ID, nil, headers)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected 204 deleting streamer, got %d", resp.Code)
	}

	monitor := performRequest(env.handler, http.MethodGet, "/api/admin/monitor/youtube", nil, headers)
	if monitor.Code != http.StatusOK {
		t.Fatalf("expected 200 fetching monitor, got %d", monitor.Code)
	}

	var payload struct {
		Events []struct {
			Mode      string `json:"mode"`
			ChannelID string `json:"channelId"`
			Status    string `json:"status"`
		}
	}
	if err := json.Unmarshal(monitor.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode monitor: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(payload.Events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(payload.Events))
	}
	if len(calls) < 2 {
		t.Fatalf("expected at least 2 webhook calls, got %d", len(calls))
	}

	last := payload.Events[len(payload.Events)-1]
	if last.Mode != "unsubscribe" {
		t.Fatalf("expected last event to be unsubscribe, got %s", last.Mode)
	}
	if last.ChannelID != created.Platforms[0].ID {
		t.Fatalf("expected channel %s, got %s", created.Platforms[0].ID, last.ChannelID)
	}
	if !strings.Contains(last.Status, "202") && !strings.Contains(last.Status, "200") {
		t.Fatalf("expected success status, got %s", last.Status)
	}
	if calls[len(calls)-1].Get("hub.mode") != "unsubscribe" {
		t.Fatalf("expected unsubscribe mode in webhook, got %s", calls[len(calls)-1].Get("hub.mode"))
	}
}
