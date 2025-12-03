package service

import (
	"context"
	"errors"
	"github.com/Its-donkey/Sharpen-live/internal/alert/streamers"
	"github.com/Its-donkey/Sharpen-live/internal/alert/submissions"
	"path/filepath"
	"testing"
	"time"
)

func TestSubmissionsServiceReject(t *testing.T) {
	dir := t.TempDir()
	subStore := submissions.NewStore(filepath.Join(dir, "subs.json"))
	if _, err := subStore.Append(submissions.Submission{ID: "sub_1", Alias: "Test", SubmittedAt: time.Now()}); err != nil {
		t.Fatalf("append submission: %v", err)
	}
	streamStore := streamers.NewStore(filepath.Join(dir, "streamers.json"))
	svc := NewSubmissionsService(SubmissionsOptions{
		SubmissionsStore: subStore,
		StreamersStore:   streamStore,
	})
	result, err := svc.Process(context.Background(), ActionRequest{Action: ActionReject, ID: "sub_1"})
	if err != nil {
		t.Fatalf("process reject: %v", err)
	}
	if result.Status != ActionReject {
		t.Fatalf("expected reject status, got %s", result.Status)
	}
	remaining, err := subStore.List()
	if err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected submissions cleared, got %d", len(remaining))
	}
}

func TestSubmissionsServiceApprove(t *testing.T) {
	dir := t.TempDir()
	subStore := submissions.NewStore(filepath.Join(dir, "subs.json"))
	pending, err := subStore.Append(submissions.Submission{
		ID:    "sub_1",
		Alias: "Test",
		Platforms: map[string]submissions.PlatformInfo{
			"youtube": {URL: "https://youtube.com/channel/UC123", ChannelID: "UC123"},
		},
		SubmittedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("append submission: %v", err)
	}
	streamStore := streamers.NewStore(filepath.Join(dir, "streamers.json"))
	svc := NewSubmissionsService(SubmissionsOptions{
		SubmissionsStore: subStore,
		StreamersStore:   streamStore,
	})
	result, err := svc.Process(context.Background(), ActionRequest{Action: ActionApprove, ID: pending.ID})
	if err != nil {
		t.Fatalf("process approval: %v", err)
	}
	if result.Status != ActionApprove {
		t.Fatalf("expected approve status, got %s", result.Status)
	}
	records, err := streamStore.List()
	if err != nil {
		t.Fatalf("list streamers: %v", err)
	}
	if len(records) != 1 || records[0].Streamer.Alias != "Test" {
		t.Fatalf("expected streamer appended, got %+v", records)
	}
}

func TestSubmissionsServiceDuplicateAlias(t *testing.T) {
	dir := t.TempDir()
	subStore := submissions.NewStore(filepath.Join(dir, "subs.json"))
	if _, err := subStore.Append(submissions.Submission{ID: "sub_1", Alias: "Test", SubmittedAt: time.Now()}); err != nil {
		t.Fatalf("append submission: %v", err)
	}
	streamStore := streamers.NewStore(filepath.Join(dir, "streamers.json"))
	if _, err := streamStore.Append(streamers.Record{Streamer: streamers.Streamer{Alias: "Test"}}); err != nil {
		t.Fatalf("seed streamers: %v", err)
	}
	svc := NewSubmissionsService(SubmissionsOptions{
		SubmissionsStore: subStore,
		StreamersStore:   streamStore,
	})
	if _, err := svc.Process(context.Background(), ActionRequest{Action: ActionApprove, ID: "sub_1"}); err == nil {
		t.Fatalf("expected duplicate alias error")
	}
	list, err := subStore.List()
	if err != nil {
		t.Fatalf("list submissions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected submission requeued, got %d", len(list))
	}
}

func TestSubmissionsServiceValidation(t *testing.T) {
	svc := NewSubmissionsService(SubmissionsOptions{
		SubmissionsStore: submissions.NewStore(filepath.Join(t.TempDir(), "subs.json")),
		StreamersStore:   streamers.NewStore(filepath.Join(t.TempDir(), "stream.json")),
	})
	if _, err := svc.Process(context.Background(), ActionRequest{Action: "invalid", ID: "1"}); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("expected invalid action error, got %v", err)
	}
	if _, err := svc.Process(context.Background(), ActionRequest{Action: ActionApprove}); !errors.Is(err, ErrMissingIdentifier) {
		t.Fatalf("expected missing id error, got %v", err)
	}
	if _, err := svc.Process(context.Background(), ActionRequest{Action: ActionReject, ID: "nope"}); !errors.Is(err, submissions.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
