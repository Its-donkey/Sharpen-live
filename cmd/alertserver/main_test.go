package main

import "testing"

func TestNormalizeSiteKey(t *testing.T) {
	tests := []struct {
		name     string
		site     string
		baseName string
		want     string
	}{
		{name: "empty", site: "", baseName: "Sharpen.Live", want: ""},
		{name: "base alias", site: "sharpen-live", baseName: "Sharpen.Live", want: ""},
		{name: "base name match", site: "Sharpen.Live", baseName: "Sharpen.Live", want: ""},
		{name: "default keyword", site: "default", baseName: "Sharpen.Live", want: ""},
		{name: "alternate site", site: "synth-wave", baseName: "Sharpen.Live", want: "synth-wave"},
		{name: "trimmed", site: "  synth-wave  ", baseName: "Sharpen.Live", want: "synth-wave"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSiteKey(tt.site, tt.baseName); got != tt.want {
				t.Fatalf("normalizeSiteKey(%q, %q) = %q, want %q", tt.site, tt.baseName, got, tt.want)
			}
		})
	}
}
