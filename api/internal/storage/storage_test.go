package storage_test

import (
	"path/filepath"
	"testing"

	"github.com/Its-donkey/Sharpen-live/api/internal/storage"
)

func newStore(t *testing.T) *storage.JSONStore {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewJSONStore(
		filepath.Join(dir, "streamers.json"),
		filepath.Join(dir, "submissions.json"),
	)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return store
}

func TestJSONStoreStreamersLifecycle(t *testing.T) {
	store := newStore(t)

	streamers, err := store.ListStreamers()
	if err != nil {
		t.Fatalf("list streamers: %v", err)
	}
	if len(streamers) != 0 {
		t.Fatalf("expected empty streamers list, got %d", len(streamers))
	}

	created, err := store.CreateStreamer(storage.Streamer{
		Name:        "EdgeCrafter",
		Description: "Knife sharpening",
		Status:      "online",
		StatusLabel: "Online",
		Languages:   []string{"English"},
		Platforms: []storage.Platform{
			{Name: "Twitch", ChannelURL: "https://example.com"},
		},
	})
	if err != nil {
		t.Fatalf("create streamer: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated streamer id")
	}

	streamers, err = store.ListStreamers()
	if err != nil {
		t.Fatalf("list streamers after create: %v", err)
	}
	if len(streamers) != 1 {
		t.Fatalf("expected 1 streamer, got %d", len(streamers))
	}

	created.Description = "Updated description"
	updated, err := store.UpdateStreamer(created)
	if err != nil {
		t.Fatalf("update streamer: %v", err)
	}
	if updated.Description != "Updated description" {
		t.Fatalf("expected updated description, got %s", updated.Description)
	}

	if err := store.DeleteStreamer(created.ID); err != nil {
		t.Fatalf("delete streamer: %v", err)
	}

	if err := store.DeleteStreamer(created.ID); err != storage.ErrNotFound {
		t.Fatalf("expected ErrNotFound when deleting again, got %v", err)
	}
}

func TestJSONStoreSubmissionsLifecycle(t *testing.T) {
	store := newStore(t)

	submission, err := store.AddSubmission(storage.SubmissionPayload{
		Name:        "New Streamer",
		Description: "Sharpening demo",
		Status:      "busy",
		StatusLabel: "Workshop",
		Languages:   []string{"English", "German"},
		Platforms: []storage.Platform{
			{Name: "YouTube", ChannelURL: "https://youtube.com/demo"},
		},
	})
	if err != nil {
		t.Fatalf("add submission: %v", err)
	}
	if submission.ID == "" {
		t.Fatal("expected generated submission id")
	}

	submissions, err := store.ListSubmissions()
	if err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(submissions) != 1 {
		t.Fatalf("expected 1 submission, got %d", len(submissions))
	}

	streamer, err := store.ApproveSubmission(submission.ID)
	if err != nil {
		t.Fatalf("approve submission: %v", err)
	}
	if streamer.ID == "" {
		t.Fatal("expected generated streamer id after approval")
	}
	if streamer.Status != "busy" {
		t.Fatalf("expected status busy, got %s", streamer.Status)
	}

	streamers, err := store.ListStreamers()
	if err != nil {
		t.Fatalf("list streamers after approval: %v", err)
	}
	if len(streamers) != 1 {
		t.Fatalf("expected 1 streamer after approval, got %d", len(streamers))
	}

	submissions, err = store.ListSubmissions()
	if err != nil {
		t.Fatalf("list submissions after approval: %v", err)
	}
	if len(submissions) != 0 {
		t.Fatalf("expected 0 submissions after approval, got %d", len(submissions))
	}

	second, err := store.AddSubmission(storage.SubmissionPayload{
		Name:        "Reject Me",
		Description: "Another demo",
		Status:      "offline",
		StatusLabel: "Offline",
		Languages:   []string{"English"},
		Platforms: []storage.Platform{
			{Name: "Kick", ChannelURL: "https://kick.com/demo"},
		},
	})
	if err != nil {
		t.Fatalf("add second submission: %v", err)
	}

	if err := store.RejectSubmission(second.ID); err != nil {
		t.Fatalf("reject submission: %v", err)
	}

	if err := store.RejectSubmission(second.ID); err != storage.ErrNotFound {
		t.Fatalf("expected ErrNotFound when rejecting missing submission, got %v", err)
	}
}
