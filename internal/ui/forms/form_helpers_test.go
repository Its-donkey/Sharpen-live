package forms

import "testing"

func TestCanonicalizeChannelInput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{name: "blank", in: "  ", out: ""},
		{name: "already https", in: "https://example.com/live", out: "https://example.com/live"},
		{name: "relative youtube", in: "youtube.com/@edge", out: "https://youtube.com/@edge"},
		{name: "handle", in: "@craft", out: "https://www.youtube.com/@craft"},
		{name: "short youtu", in: "youtu.be/demo", out: "https://youtu.be/demo"},
	}
	for _, tc := range cases {
		if got := CanonicalizeChannelInput(tc.in); got != tc.out {
			t.Fatalf("%s: expected %q got %q", tc.name, tc.out, got)
		}
	}
}

func TestDerivePlatformLabel(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
	}{
		{name: "blank", in: "", out: ""},
		{name: "handle", in: "@forge", out: "@forge"},
		{name: "url host", in: "https://twitch.tv/edge", out: "twitch.tv"},
		{name: "url handle", in: "https://youtube.com/@edge", out: "@edge"},
		{name: "plain", in: "kick", out: "kick"},
	}
	for _, tc := range cases {
		if got := DerivePlatformLabel(tc.in); got != tc.out {
			t.Fatalf("%s: expected %q got %q", tc.name, tc.out, got)
		}
	}
}

func TestAvailableLanguageOptions(t *testing.T) {
	selected := []string{"English", "Spanish"}
	options := AvailableLanguageOptions(selected)
	for _, opt := range options {
		for _, s := range selected {
			if opt.Value == s {
				t.Fatalf("option %q should be filtered", s)
			}
		}
	}
}

func TestBuildURLFromHandle(t *testing.T) {
	cases := []struct {
		handle  string
		preset  string
		wantURL string
	}{
		{handle: "@edge", preset: "youtube", wantURL: "https://www.youtube.com/@edge"},
		{handle: "@edge", preset: "twitch", wantURL: "https://www.twitch.tv/edge"},
		{handle: "@edge", preset: "facebook", wantURL: "https://www.facebook.com/edge"},
		{handle: " @edge ", preset: "unknown", wantURL: "https://www.youtube.com/@edge"},
	}
	for _, tc := range cases {
		if got := buildURLFromHandle(tc.handle, tc.preset); got != tc.wantURL {
			t.Fatalf("handle %q preset %q: want %q got %q", tc.handle, tc.preset, tc.wantURL, got)
		}
	}
}

func TestResolvePlatformPreset(t *testing.T) {
	if got := resolvePlatformPreset("TWITCH"); got != "twitch" {
		t.Fatalf("expected twitch got %q", got)
	}
	if got := resolvePlatformPreset("  "); got != "youtube" {
		t.Fatalf("blank should default to youtube, got %q", got)
	}
}
