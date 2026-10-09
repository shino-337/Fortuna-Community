package matcher

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/cve/catalogtest"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestParsePURLSpecShapes(t *testing.T) {
	cases := []struct {
		in                  string
		eco, ns, name, ver  string
		upstream, arch, dis string
	}{
		{
			in:  "pkg:deb/debian/libssl3@3.0.15-1~deb12u1?arch=amd64&distro=debian-12&upstream=openssl",
			eco: "deb", ns: "debian", name: "libssl3", ver: "3.0.15-1~deb12u1",
			upstream: "openssl", arch: "amd64", dis: "debian-12",
		},
		{
			// Epoch is percent-encoded and upstream carries its own '@version'.
			in:  "pkg:deb/debian/zlib1g@1%3A1.2.13.dfsg-1?arch=amd64&upstream=zlib%401%3A1.2.13.dfsg-1",
			eco: "deb", ns: "debian", name: "zlib1g", ver: "1:1.2.13.dfsg-1",
			upstream: "zlib@1:1.2.13.dfsg-1", arch: "amd64",
		},
		{
			// Unencoded '@' in a qualifier (Syft style) must not be taken as the version separator.
			in:  "pkg:apk/alpine/libcrypto3@3.3.2-r0?arch=x86_64&upstream=openssl@3.3.2-r0&distro=alpine-3.20.3",
			eco: "apk", ns: "alpine", name: "libcrypto3", ver: "3.3.2-r0",
			upstream: "openssl@3.3.2-r0", arch: "x86_64", dis: "alpine-3.20.3",
		},
		{in: "pkg:npm/%40babel/core@7.24.0", eco: "npm", ns: "@babel", name: "core", ver: "7.24.0"},
		{in: "pkg:npm/@babel/core@7.24.0", eco: "npm", ns: "@babel", name: "core", ver: "7.24.0"},
		{in: "pkg:pypi/django@3.2.5#src/django", eco: "pypi", name: "django", ver: "3.2.5"},
		{in: "pkg:deb/debian/g%2B%2B-12@12.2.0-14", eco: "deb", ns: "debian", name: "g++-12", ver: "12.2.0-14"},
		{in: "pkg:go/github.com/coreos/etcd/client/v3@v3.3.0", eco: "go", name: "github.com/coreos/etcd/client/v3", ver: "v3.3.0"},
	}
	for _, tc := range cases {
		p, err := ParsePURL(tc.in)
		if err != nil {
			t.Fatalf("ParsePURL(%q): %v", tc.in, err)
		}
		if p.Ecosystem != tc.eco || p.Namespace != tc.ns || p.Name != tc.name || p.Version != tc.ver {
			t.Errorf("ParsePURL(%q) = eco=%q ns=%q name=%q ver=%q", tc.in, p.Ecosystem, p.Namespace, p.Name, p.Version)
		}
		if p.Qualifiers["upstream"] != tc.upstream || p.Qualifiers["arch"] != tc.arch || p.Qualifiers["distro"] != tc.dis {
			t.Errorf("ParsePURL(%q) qualifiers = %v", tc.in, p.Qualifiers)
		}
	}
	for _, bad := range []string{"npm/lodash@1", "pkg:npm/lodash", "pkg:npm/lodash@", "pkg:lodash@1"} {
		if _, err := ParsePURL(bad); err == nil {
			t.Errorf("ParsePURL(%q) succeeded, want error", bad)
		}
	}
}

func TestDistroSourceIdentity(t *testing.T) {
	cases := []struct {
		purl, installed     string
		wantEco, wantKey    string
		wantComparedVersion string
	}{
		// Binary package keyed by its source; version stays the binary one when upstream has none.
		{"pkg:deb/debian/libssl3@3.0.15-1~deb12u1?arch=amd64&upstream=openssl", "3.0.15-1~deb12u1", "debian", "openssl", "3.0.15-1~deb12u1"},
		// binNMU binary version differs from its source version: compare with the source one.
		{"pkg:deb/debian/libfoo1@1.2-3%2Bb1?upstream=foo%401.2-3", "1.2-3+b1", "debian", "foo", "1.2-3"},
		// Source == binary: no upstream qualifier, purl name is the key.
		{"pkg:deb/ubuntu/bash@5.1-6ubuntu1?arch=amd64&distro=ubuntu-22.04", "5.1-6ubuntu1", "ubuntu", "bash", "5.1-6ubuntu1"},
		// Alpine subpackage keyed by origin.
		{"pkg:apk/alpine/libcrypto3@3.3.2-r0?upstream=openssl", "3.3.2-r0", "alpine", "openssl", "3.3.2-r0"},
		// Distroless namespace queries Debian data.
		{"pkg:deb/distroless/libc6@2.36-9", "2.36-9", "debian", "libc6", "2.36-9"},
	}
	for _, tc := range cases {
		p, err := ParsePURL(tc.purl)
		if err != nil {
			t.Fatal(err)
		}
		if got := normalizeQueryEcosystemWithOS(p, ""); got != tc.wantEco {
			t.Errorf("%s: ecosystem = %q, want %q", tc.purl, got, tc.wantEco)
		}
		key := p.Name
		if src, _ := distroSourcePackage(p); src != "" {
			key = src
		}
		if !isDistroSourceKeyed(p) || key != tc.wantKey {
			t.Errorf("%s: key = %q (source keyed=%v), want %q", tc.purl, key, isDistroSourceKeyed(p), tc.wantKey)
		}
		if got := effectiveVersionForComparison(tc.installed, p); got != tc.wantComparedVersion {
			t.Errorf("%s: compared version = %q, want %q", tc.purl, got, tc.wantComparedVersion)
		}
	}
}

func TestLanguageEcosystemsIgnoreNamespace(t *testing.T) {
	cases := map[string]string{
		"pkg:maven/org.apache.tomcat/tomcat-catalina@10.1.30": "maven",
		"pkg:npm/%40babel/core@7.24.0":                        "npm",
		"pkg:gem/rails@7.1.0":                                 "rubygems",
		"pkg:composer/laravel/framework@10.0.0":               "packagist",
		"pkg:cargo/serde@1.0.0":                               "cargo",
	}
	for in, want := range cases {
		p, err := ParsePURL(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := normalizeQueryEcosystemWithOS(p, ""); got != want {
			t.Errorf("%s: ecosystem = %q, want %q", in, got, want)
		}
	}
}

// A Debian binary package (libssl3) must match advisories filed against its source
// package (openssl) through the purl `upstream` qualifier.
func TestMatchSBOM_DebianBinaryMatchesSourceAdvisory(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	catalogtest.Seed(t, db, catalogtest.Advisory{ID: "CVE-2025-15467", Severity: "HIGH", Affected: []catalogtest.Range{
		{Ecosystem: "debian", Package: "openssl", Fixed: "3.0.19-1"},
	}})
	sbom := &models.SBOM{OSName: "debian", OSVersion: "12", Status: "finalized"}
	if err := db.Create(sbom).Error; err != nil {
		t.Fatalf("create SBOM: %v", err)
	}
	for _, c := range []models.SBOMComponent{
		{SBOMID: sbom.ID, ComponentName: "libssl3", ComponentVersion: "3.0.18-1~deb12u2", ComponentType: "library",
			PURL: "pkg:deb/debian/libssl3@3.0.18-1~deb12u2?arch=amd64&distro=debian-12&upstream=openssl"},
		{SBOMID: sbom.ID, ComponentName: "openssl", ComponentVersion: "3.0.19-1", ComponentType: "library",
			PURL: "pkg:deb/debian/openssl@3.0.19-1?arch=amd64&distro=debian-12"},
	} {
		c := c
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("create component: %v", err)
		}
	}

	matches, err := NewMatcher(database.NewPostgresManager(db), db).MatchSBOM(context.Background(), sbom, nil)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	var vulnerable, fixed bool
	for _, m := range matches {
		if m.CVEID != "CVE-2025-15467" {
			continue
		}
		switch m.PackageVersion {
		case "3.0.18-1~deb12u2":
			vulnerable = true
		case "3.0.19-1":
			fixed = true
		}
	}
	if !vulnerable {
		t.Errorf("libssl3 (upstream=openssl) should match the openssl advisory; got %+v", matches)
	}
	if fixed {
		t.Errorf("openssl 3.0.19-1 is the fixed version and must not match; got %+v", matches)
	}
}
