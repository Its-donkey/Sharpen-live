package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestEnsureYouTubeSubscription_SendsRequest(t *testing.T) {
	var (
		requestCount int
		requestBody  string
		requestPath  string
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		requestPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			t.Fatalf("unexpected content type: %s", ct)
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		requestBody = string(data)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	srv := New(nil, "", "", "", "", WithYouTubeAlerts(YouTubeAlertsConfig{
		HubURL:            ts.URL,
		CallbackURL:       "https://sharpen.live/alerts",
		Secret:            "sharpen-secret",
		VerifyTokenPrefix: "sharpen-",
		VerifyTokenSuffix: "-testing",
	}))
	srv.httpClient = ts.Client()

	srv.ensureYouTubeSubscription(context.Background(), "UC123")

	if requestCount != 1 {
		t.Fatalf("expected 1 request, got %d", requestCount)
	}
	if requestPath != "" && requestPath != "/" {
		t.Fatalf("unexpected request path: %s", requestPath)
	}

	values, err := url.ParseQuery(requestBody)
	if err != nil {
		t.Fatalf("parse body: %v", err)
	}

	if got := values.Get("hub.mode"); got != "subscribe" {
		t.Fatalf("unexpected hub.mode: %s", got)
	}
	expectedTopic := "https://www.youtube.com/xml/feeds/videos.xml?channel_id=UC123"
	if got := values.Get("hub.topic"); got != expectedTopic {
		t.Fatalf("unexpected hub.topic: %s", got)
	}
	if got := values.Get("hub.callback"); got != "https://sharpen.live/alerts" {
		t.Fatalf("unexpected hub.callback: %s", got)
	}
	if got := values.Get("hub.verify"); got != "async" {
		t.Fatalf("unexpected hub.verify: %s", got)
	}
	if got := values.Get("hub.verify_token"); got != "sharpen-UC123-testing" {
		t.Fatalf("unexpected hub.verify_token: %s", got)
	}
	if got := values.Get("hub.secret"); got != "sharpen-secret" {
		t.Fatalf("unexpected hub.secret: %s", got)
	}
}

func TestEnsureYouTubeSubscription_SkipsWhenDisabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
	}))
	defer ts.Close()

	srv := New(nil, "", "", "", "")
	srv.httpClient = ts.Client()
	srv.youtubeHubURL = ts.URL

	srv.ensureYouTubeSubscription(context.Background(), "UC123")
}
