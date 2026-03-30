package extractor

import (
	"strings"
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
			// Finding #8.2: synthetic must have PURL and source
			if p.PURL == "" || !strings.HasPrefix(p.PURL, "pkg:generic/") {
				t.Errorf("PURL = %q, want pkg:generic/...", p.PURL)
			}
			if p.Source != "distroless-heuristic" {
				t.Errorf("Source = %q, want distroless-heuristic", p.Source)
			}
			// Version from ref tag → confidence medium for NVD fallback
			if tt.wantVer != "unknown" && p.Confidence != "medium" {
				t.Errorf("confidence with version from tag: got %q, want medium", p.Confidence)
			}
		})
	}
}

// TestDistrolessSkipBasename verifies distroless parser skips os-release, build.log, *.log, .pl, non-lib .so (no SBOM garbage).
func TestDistrolessSkipBasename(t *testing.T) {
	skip := []string{"os-release", "build.log", "a.log", "build.log", "Extend.pl", "Y.pl", "ISO-IR-197.so"}
	keep := []string{"hello", "kube-apiserver", "busybox", "libc.so.6", "libssl.so.3", "coredns"}
	for _, b := range skip {
		if !DistrolessSkipBasename(b) {
			t.Errorf("DistrolessSkipBasename(%q) = false, want true (skip)", b)
		}
	}
	for _, b := range keep {
		if DistrolessSkipBasename(b) {
			t.Errorf("DistrolessSkipBasename(%q) = true, want false (keep)", b)
		}
	}
}

// TestDistrolessParserSkipsJunk verifies .pl, charset .so (ISO-IR-197.so), and share/locale are not emitted as packages.
func TestDistrolessParserSkipsJunk(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/usr/bin/coredns"] = []byte{}
	fs.files["/usr/lib/Extend.pl"] = []byte{}
	fs.files["/usr/lib/ISO-IR-197.so"] = []byte{}
	fs.files["/usr/lib/share/locale/en/LC_MESSAGES/foo.mo"] = []byte{}
	p := NewDistrolessParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	names := make(map[string]bool)
	for _, pkg := range pkgs {
		names[pkg.Name] = true
	}
	if !names["coredns"] {
		t.Error("expected coredns to be kept (control-plane whitelist)")
	}
	if names["Extend.pl"] || names["ISO-IR-197.so"] {
		t.Errorf("junk packages should be skipped: got %v", names)
	}
}

// TestSyntheticPackageDistrolessHello verifies index.docker.io/library/distroless-hello:e2e → distroless-hello@e2e, confidence medium.
func TestSyntheticPackageDistrolessHello(t *testing.T) {
	e := NewExtractor()
	p := e.syntheticPackageFromImage("index.docker.io/library/distroless-hello:e2e", nil)
	if p.Name != "distroless-hello" {
		t.Errorf("Name = %q, want distroless-hello", p.Name)
	}
	if p.Version != "e2e" {
		t.Errorf("Version = %q, want e2e", p.Version)
	}
	if p.PURL != "pkg:generic/distroless-hello@e2e" {
		t.Errorf("PURL = %q, want pkg:generic/distroless-hello@e2e", p.PURL)
	}
	if p.Source != "distroless-heuristic" {
		t.Errorf("Source = %q, want distroless-heuristic", p.Source)
	}
	if p.Confidence != "medium" {
		t.Errorf("confidence = %q, want medium (NVD fallback)", p.Confidence)
	}
}

func TestDistrolessParserUsesSignatures(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/usr/bin/coredns"] = []byte{}
	p := NewDistrolessParser()
	pkgs, err := p.Parse(fs)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d", len(pkgs))
	}
	if pkgs[0].PURL != "pkg:golang/github.com/coredns/coredns@unknown" {
		t.Fatalf("PURL = %q, want pkg:golang/github.com/coredns/coredns@unknown", pkgs[0].PURL)
	}
	if pkgs[0].Confidence != "high" {
		t.Fatalf("Confidence = %q, want high", pkgs[0].Confidence)
	}
}

func TestSyntheticPackageFromImageRef_Invalid(t *testing.T) {
	e := NewExtractor()
	p := e.syntheticPackageFromImage("", nil)
	if p.Name != "unknown-image" || p.Version != "unknown" || p.Type != "generic" {
		t.Errorf("empty ref should yield unknown-image@unknown: got %s@%s type=%s", p.Name, p.Version, p.Type)
	}
	if p.PURL != "pkg:generic/unknown-image@unknown" || p.Source != "distroless-heuristic" {
		t.Errorf("invalid ref: PURL=%q Source=%q", p.PURL, p.Source)
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
	if p.PURL != "pkg:generic/coredns@v1.10.0" {
		t.Errorf("PURL from OCI label version: got %q", p.PURL)
	}
	if p.Confidence != "medium" {
		t.Errorf("confidence when version from label: got %q, want medium", p.Confidence)
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

// TestDistrolessParser verifies the distroless parser emits only binaries in the signature DB (control-plane allowlist).
// Binaries not in the signature (busybox, nginx, libc.so) are not emitted to avoid SBOM junk.
func TestDistrolessParser(t *testing.T) {
	fs := NewFilesystem()
	fs.files["/bin/busybox"] = []byte{}
	fs.files["/usr/bin/nginx"] = []byte{}
	fs.files["/usr/bin/coredns"] = []byte{} // in signature → emitted
	fs.files["/usr/lib/libc.so"] = []byte{}
	fs.files["/etc/os-release"] = []byte{} // should be ignored (not under bin/lib)

	p := NewDistrolessParser()
	packages, err := p.Parse(fs)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Only coredns is in distroless.json; busybox, nginx, libc.so are not → 1 package
	if len(packages) != 1 {
		t.Errorf("got %d packages, want 1 (only coredns in signature); names=%v", len(packages), pkgNames(packages))
	}
	for _, pkg := range packages {
		if pkg.Name != "coredns" {
			t.Errorf("unexpected package %q (only signature-listed binaries should be emitted)", pkg.Name)
		}
		if pkg.Type != "generic" || pkg.Source != "distroless-heuristic" || strings.TrimSpace(pkg.PURL) == "" {
			t.Errorf("package %s: Type=%q Source=%q PURL=%q", pkg.Name, pkg.Type, pkg.Source, pkg.PURL)
		}
	}
}

func pkgNames(pkgs []Package) []string {
	names := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		names = append(names, p.Name)
	}
	return names
}

// TestDistrolessParserJunkNotEmitted verifies OS utility binaries (setpriv, ln, mkdir, rm, ...) are not
// emitted as SBOM components when they exist in the image. Only control-plane binaries in the signature are emitted.
func TestDistrolessParserJunkNotEmitted(t *testing.T) {
	fs := NewFilesystem()
	junk := []string{"setpriv", "zless", "resizepart", "addpart", "dircolors", "ln", "script", "expand", "mkdir", "mesg", "toe", "gpgv", "rm", "perl", "dpkg-deb", "groups", "install", "rmdir", "docker", "egrep", "umount", "prlimit"}
	for _, name := range junk {
		fs.files["/usr/bin/"+name] = []byte{}
	}
	fs.files["/usr/bin/kube-apiserver"] = []byte{} // in signature

	p := NewDistrolessParser()
	packages, err := p.Parse(fs)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	names := pkgNames(packages)
	if len(packages) != 1 {
		t.Errorf("got %d packages, want 1 (only kube-apiserver); got %v", len(packages), names)
	}
	if len(packages) > 0 && packages[0].Name != "kube-apiserver" {
		t.Errorf("only kube-apiserver should be emitted, got %v", names)
	}
	for _, pkg := range packages {
		for _, j := range junk {
			if pkg.Name == j {
				t.Errorf("junk binary %q must not be emitted as SBOM component", j)
			}
		}
	}
}

// TestSetOSPackagePURLs verifies dpkg/apk packages get pkg:deb/<distro>/name@version for OSV/CVE query.
func TestSetOSPackagePURLs(t *testing.T) {
	osDebian := OSInfo{Name: "debian", Version: "12"}
	osUbuntu := OSInfo{Name: "ubuntu", Version: "22.04"}
	osAlpine := OSInfo{Name: "alpine", Version: "3.18"}
	tests := []struct {
		name string
		os   OSInfo
		pkgs []Package
		want []string // PURL per package (same order as pkgs after filter)
	}{
		{
			name: "debian openssl",
			os:   osDebian,
			pkgs: []Package{{Name: "openssl", Version: "3.0.18-1~deb12u2", Type: "deb", PURL: ""}},
			want: []string{"pkg:deb/debian/openssl@3.0.18-1~deb12u2"},
		},
		{
			name: "ubuntu libc6",
			os:   osUbuntu,
			pkgs: []Package{{Name: "libc6", Version: "2.35-0ubuntu3", Type: "deb", PURL: ""}},
			want: []string{"pkg:deb/ubuntu/libc6@2.35-0ubuntu3"},
		},
		{
			name: "alpine apk",
			os:   osAlpine,
			pkgs: []Package{{Name: "alpine-baselayout", Version: "3.4.3-r2", Type: "apk", PURL: ""}},
			want: []string{"pkg:apk/alpine/alpine-baselayout@3.4.3-r2"},
		},
		{
			name: "skip when PURL already set",
			os:   osDebian,
			pkgs: []Package{{Name: "openssl", Version: "1.1.1", Type: "deb", PURL: "pkg:deb/debian/openssl@1.1.1"}},
			want: []string{"pkg:deb/debian/openssl@1.1.1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setOSPackagePURLs(tt.pkgs, tt.os)
			for i, w := range tt.want {
				if i >= len(got) || got[i].PURL != w {
					t.Errorf("package[%d] PURL = %q, want %q", i, func() string {
						if i < len(got) {
							return got[i].PURL
						}
						return ""
					}(), w)
				}
			}
		})
	}
}
