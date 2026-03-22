package sbom

import "testing"

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		in       string
		wantName string
		wantTag  string
	}{
		{"nginx:1.25", "nginx", "1.25"},
		{"registry:5000/repo/nginx:1.25", "registry:5000/repo/nginx", "1.25"},
		{"registry:5000/repo/nginx", "registry:5000/repo/nginx", "latest"},
		{"registry:5000/nginx", "registry:5000/nginx", "latest"},
		{"repo/nginx@sha256:abc123", "repo/nginx", "sha256:abc123"},
		{"  repo/nginx:2.0  ", "repo/nginx", "2.0"},
		{"image:", "image", "latest"},
		{"nginx", "nginx", "latest"},
		{"", "", "latest"},
	}

	for _, tt := range tests {
		gotName, gotTag := parseImageRef(tt.in)
		if gotName != tt.wantName || gotTag != tt.wantTag {
			t.Fatalf("parseImageRef(%q)=(%q,%q), want (%q,%q)", tt.in, gotName, gotTag, tt.wantName, tt.wantTag)
		}
	}
}

