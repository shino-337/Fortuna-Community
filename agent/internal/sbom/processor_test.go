package sbom

import (
	"testing"
)

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		name      string
		imageRef  string
		wantName  string
		wantTag   string
	}{
		{
			name:     "registry with port and tag",
			imageRef: "registry.example.com:5000/ns/img:v1.0",
			wantName: "registry.example.com:5000/ns/img",
			wantTag:  "v1.0",
		},
		{
			name:     "digest reference",
			imageRef: "nginx@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantName: "index.docker.io/library/nginx",
			wantTag:  "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple tag",
			imageRef: "nginx:latest",
			wantName: "index.docker.io/library/nginx",
			wantTag:  "latest",
		},
		{
			name:     "no tag defaults to latest",
			imageRef: "nginx",
			wantName: "index.docker.io/library/nginx",
			wantTag:  "latest",
		},
		{
			name:     "qualified repo with tag",
			imageRef: "gcr.io/myproject/myimage:tag",
			wantName: "gcr.io/myproject/myimage",
			wantTag:  "tag",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotTag := parseImageRef(tt.imageRef)
			if gotName != tt.wantName {
				t.Errorf("parseImageRef(%q) name = %q, want %q", tt.imageRef, gotName, tt.wantName)
			}
			if gotTag != tt.wantTag {
				t.Errorf("parseImageRef(%q) tag = %q, want %q", tt.imageRef, gotTag, tt.wantTag)
			}
		})
	}
}
