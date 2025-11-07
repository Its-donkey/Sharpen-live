package server

import (
	"path/filepath"
	"testing"
)

func TestLoadStreamerDirectory(t *testing.T) {
	file := filepath.Join("..", "..", "..", "..", "data", "streamers.json")
	dir, err := LoadStreamerDirectory(file)
	if err != nil {
		t.Fatalf("LoadStreamerDirectory error: %v", err)
	}

	if len(dir) == 0 {
		t.Fatal("expected streamers directory to be populated")
	}
}

func TestChannelIDFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/channel/UC123":    "UC123",
		"https://youtube.com/xml?channel_id=UC456": "UC456",
	}

	for input, expected := range cases {
		if got := channelIDFromURL(input); got != expected {
			t.Fatalf("channelIDFromURL(%q)=%q want %q", input, got, expected)
		}
	}
}
