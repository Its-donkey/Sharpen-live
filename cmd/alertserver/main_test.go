package main

import "testing"

func TestNormalizeSiteKey(t *testing.T) {
	tests := []struct {
		name string
		site string
		want string
	}{
		{name: "empty", site: "", want: ""},
		{name: "base alias remains", site: "sharpen-live", want: "sharpen-live"},
		{name: "base name preserved", site: "Sharpen.Live", want: "sharpen-live"},
		{name: "default keyword kept", site: "default", want: "default"},
		{name: "alternate site", site: "synth-wave", want: "synth-wave"},
		{name: "trimmed", site: "  synth wave  ", want: "synth-wave"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSiteKey(tt.site); got != tt.want {
				t.Fatalf("normalizeSiteKey(%q) = %q, want %q", tt.site, got, tt.want)
			}
		})
	}
}
