package main

import (
	"testing"

	apiv1 "github.com/Its-donkey/Sharpen-live/internal/alert/api/v1"
	"github.com/Its-donkey/Sharpen-live/internal/alert/config"
	"github.com/Its-donkey/Sharpen-live/internal/alert/logging"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
)

func TestRouterConstructionMatchesMainConfig(t *testing.T) {
	logger := logging.New()
	opts := apiv1.Options{
		Logger:        logger,
		StreamersPath: streamers.DefaultFilePath,
		YouTube: config.YouTubeConfig{
			HubURL:       "https://hub.example.com",
			CallbackURL:  "https://callback.example.com/alerts",
			Verify:       "async",
			LeaseSeconds: 60,
		},
	}
	if router := apiv1.NewRouter(opts); router == nil {
		t.Fatalf("expected router to be constructed")
	}
}
