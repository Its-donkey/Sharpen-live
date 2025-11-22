package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
)

func TestAdminAuthorizationRequiresValidToken(t *testing.T) {
	streamersFile := filepath.Join(t.TempDir(), "streamers.json")
	if err := os.WriteFile(streamersFile, []byte(`{"streamers":[]}`), 0o644); err != nil {
		t.Fatalf("write streamers file: %v", err)
	}

	router := NewRouter(Options{
		StreamersPath: streamersFile,
		Admin: config.AdminConfig{
			Email:           "admin@sharpen.live",
			Password:        "change-me",
			TokenTTLSeconds: 3600,
		},
	})

	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	client := srv.Client()

	assertStatus := func(t *testing.T, resp *http.Response, want int) {
		t.Helper()
		if resp.StatusCode != want {
			t.Fatalf("unexpected status: got %d, want %d", resp.StatusCode, want)
		}
	}

	resp, err := client.Get(srv.URL + "/api/admin/settings")
	if err != nil {
		t.Fatalf("request without token: %v", err)
	}
	resp.Body.Close()
	assertStatus(t, resp, http.StatusUnauthorized)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/admin/settings?token=bad", nil)
	if err != nil {
		t.Fatalf("build request with invalid token: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request with invalid token: %v", err)
	}
	resp.Body.Close()
	assertStatus(t, resp, http.StatusUnauthorized)

	loginResp, err := client.Post(srv.URL+"/api/admin/login", "application/json", strings.NewReader(`{"email":"admin@sharpen.live","password":"change-me"}`))
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer loginResp.Body.Close()
	assertStatus(t, loginResp, http.StatusOK)
	var login struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&login); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if login.Token == "" {
		t.Fatalf("expected login token")
	}

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/api/admin/settings?token="+login.Token, nil)
	if err != nil {
		t.Fatalf("build request with query token: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request with query token: %v", err)
	}
	resp.Body.Close()
	assertStatus(t, resp, http.StatusOK)

	req, err = http.NewRequest(http.MethodGet, srv.URL+"/api/admin/settings", nil)
	if err != nil {
		t.Fatalf("build request with bearer token: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+login.Token)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("request with bearer token: %v", err)
	}
	resp.Body.Close()
	assertStatus(t, resp, http.StatusOK)
}
