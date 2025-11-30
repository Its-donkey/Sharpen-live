package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestParseSubmitFormSupportsCurrentFormFields(t *testing.T) {
	form := url.Values{
		"name":         {"Creator"},
		"description":  {"Streams"},
		"platform_id":  {"row-1", "row-2"},
		"platform_url": {"https://youtube.com/@creator", "@handle"},
		"languages":    {"English", "French"},
		"remove_platform": {
			"row-2",
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	state, removed, err := parseSubmitForm(req)
	if err != nil {
		t.Fatalf("parseSubmitForm error: %v", err)
	}
	if state.Name != "Creator" || state.Description != "Streams" {
		t.Fatalf("expected basic fields to map, got %+v", state)
	}
	if len(state.Platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(state.Platforms))
	}
	if state.Platforms[0].ID != "row-1" || state.Platforms[0].ChannelURL == "" {
		t.Fatalf("expected first platform to retain id/url, got %+v", state.Platforms[0])
	}
	if len(removed) != 1 || removed[0] != "row-2" {
		t.Fatalf("expected removed platform IDs, got %+v", removed)
	}

	for _, id := range removed {
		state.Platforms = removePlatformRow(state.Platforms, id)
	}
	if len(state.Platforms) != 1 || state.Platforms[0].ID != "row-1" {
		t.Fatalf("expected removal to apply, remaining: %+v", state.Platforms)
	}
	if len(state.Languages) != 2 || state.Languages[0] != "English" || state.Languages[1] != "French" {
		t.Fatalf("expected languages to parse, got %+v", state.Languages)
	}
}
