package extractor

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

func TestSyntheticPackageFromImageRef(t *testing.T) {
	e := NewExtractor()

	tests := []struct {
		imageRef   string
		wantName   string
		wantVer    string
		wantType   string
	}{
		{"registry.k8s.io/coredns/coredns:v1.10.0", "coredns", "v1.10.0", "generic"},
		{"docker.io/library/nginx:latest", "nginx", "latest", "generic"},
		{"gcr.io/myproject/fortuna-core:dev", "fortuna-core", "dev", "generic"},
		{"fortuna-agent@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "fortuna-agent", "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "generic"},
		{"busybox", "busybox", "latest", "generic"},
	}
	for _, tt := range tests {
		t.Run(tt.imageRef, func(t *testing.T) {
			p := e.syntheticPackageFromImage(tt.imageRef, nil)
			if p.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", p.Name, tt.wantName)
			}
			if p.Version != tt.wantVer {
				t.Errorf("Version = %q, want %q", p.Version, tt.wantVer)
			}
			if p.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", p.Type, tt.wantType)
			}
		})
	}
}

func TestSyntheticPackageFromImageRef_Invalid(t *testing.T) {
	e := NewExtractor()
	p := e.syntheticPackageFromImage("", nil)
	if p.Name != "unknown-image" || p.Version != "unknown" || p.Type != "generic" {
		t.Errorf("empty ref should yield unknown-image@unknown: got %s@%s type=%s", p.Name, p.Version, p.Type)
	}
}

// TestSyntheticPackageFromImage_OCILabel verifies version from OCI label when no tag in ref (Finding #8.1).
func TestSyntheticPackageFromImage_OCILabel(t *testing.T) {
	e := NewExtractor()
	config := &v1.ConfigFile{
		Config: v1.Config{
			Labels: map[string]string{labelRefName: "coredns:v1.10.0"},
		},
	}
	// Ref with digest only (no tag); version should come from OCI label
	p := e.syntheticPackageFromImage("registry.k8s.io/coredns/coredns@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", config)
	if p.Version != "v1.10.0" {
		t.Errorf("version from OCI label: got %q, want v1.10.0", p.Version)
	}
	if p.Name != "coredns" {
		t.Errorf("name: got %q, want coredns", p.Name)
	}
}

// TestDetectOS_OCIDistroless verifies detectOS returns distroless when OCI label present (Finding #8.1).
func TestDetectOS_OCIDistroless(t *testing.T) {
	e := NewExtractor()
	fs := NewFilesystem() // empty FS -> no os-release
	config := &v1.ConfigFile{
		Config: v1.Config{
			Labels: map[string]string{
				"org.opencontainers.image.ref.name": "v1.0",
				"foo": "distroless",
			},
		},
	}
	osInfo := e.detectOS(fs, config)
	if osInfo.Name != "distroless" {
		t.Errorf("detectOS with distroless label: Name = %q, want distroless", osInfo.Name)
	}
	if osInfo.Version != "v1.0" {
		t.Errorf("detectOS: Version = %q, want v1.0", osInfo.Version)
	}
}

// TestIsDistrolessFromOSRelease verifies distroless detection from /etc/os-release content.
func TestIsDistrolessFromOSRelease(t *testing.T) {
	tests := []struct {
		content string
		want    bool
	}{
		{`PRETTY_NAME="Distroless"
NAME="Debian GNU/Linux"
ID="debian"
VERSION_ID="12"`, true},
		{`PRETTY_NAME="Distroless"`, true},
		{`NAME="distroless"`, true},
		{`ID=debian
VERSION_ID=9`, false},
		{`PRETTY_NAME="Ubuntu 22.04"`, false},
	}
	for i, tt := range tests {
		got := isDistrolessFromOSRelease(tt.content)
		if got != tt.want {
			t.Errorf("case %d: isDistrolessFromOSRelease = %v, want %v", i, got, tt.want)
		}
	}
}

// TestDetectOS_OSReleaseDistroless verifies detectOS returns distroless when /etc/os-release has PRETTY_NAME=Distroless.
func TestDetectOS_OSReleaseDistroless(t *testing.T) {
	e := NewExtractor()
	fs := NewFilesystem()
	// Simulate Google distroless os-release
	fs.files["/etc/os-release"] = []byte(`PRETTY_NAME="Distroless"
NAME="Debian GNU/Linux"
ID="debian"
VERSION_ID="12"
`)
	osInfo := e.detectOS(fs, nil)
	if osInfo.Name != "distroless" {
		t.Errorf("detectOS with os-release PRETTY_NAME=Distroless: Name = %q, want distroless", osInfo.Name)
	}
	if osInfo.Version != "12" {
		t.Errorf("Version = %q, want 12", osInfo.Version)
	}
}
