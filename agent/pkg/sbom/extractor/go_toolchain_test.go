package extractor

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func TestNormalizeGoToolchainVersionForSBOM(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"go1.22.5", "go1.22.5"},
		{"1.22.5", "go1.22.5"},
		{"  1.21.0  ", "go1.21.0"},
	}
	for _, tt := range tests {
		got := normalizeGoToolchainVersionForSBOM(tt.in)
		if got != tt.want {
			t.Fatalf("normalizeGoToolchainVersionForSBOM(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestGoVersionFromImageConfig(t *testing.T) {
	cfg := &v1.ConfigFile{
		Config: v1.Config{
			Env: []string{
				"PATH=/usr/local/go/bin",
				"GOLANG_VERSION=1.22.5",
			},
		},
	}
	got := goVersionFromImageConfig(cfg)
	if got != "1.22.5" {
		t.Fatalf("got %q want 1.22.5", got)
	}
	got2 := normalizeGoToolchainVersionForSBOM(got)
	if got2 != "go1.22.5" {
		t.Fatalf("normalized: got %q", got2)
	}
}
