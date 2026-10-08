package matcher

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/mirror/osv"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOpenAndExactConstraints(t *testing.T) {
	vc := NewVersionComparator()
	cases := []struct {
		installed, constraint, eco string
		want                       bool
	}{
		{"3.0.15-1~deb12u1", ">=0", "debian", true},
		{"1:1.2.13.dfsg-1", ">=0", "debian", true},
		{"3.3.2-r0", ">=0", "alpine", true},
		{"1.2.3", ">=0", "npm", true},
		{"1.1", ">=1.1, <=1.1", "pypi", true},
		{"1.2", ">=1.1, <=1.1", "pypi", false},
	}
	for _, tc := range cases {
		got, err := vc.IsVulnerable(tc.installed, tc.constraint, tc.eco)
		if err != nil || got != tc.want {
			t.Errorf("IsVulnerable(%q, %q, %s) = %v, %v; want %v", tc.installed, tc.constraint, tc.eco, got, err, tc.want)
		}
	}
}

// A Debian 12 component is matched only against Debian 12 ranges, and an advisory with no fix
// yet for Debian 12 matches.
func TestMatchSBOM_UsesRangesOfTheComponentRelease(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVE{}, &models.PackageVulnerability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	for _, id := range []string{"CVE-2025-1111", "CVE-2025-2222", "CVE-2025-3333"} {
		if err := db.Create(&models.CVE{CVEID: id, Severity: "HIGH", CVSSScore: 7.5, PublishedDate: &now, LastModifiedDate: &now}).Error; err != nil {
			t.Fatalf("create CVE: %v", err)
		}
	}
	rows := []models.PackageVulnerability{
		// 1111: fixed in Debian 12 below the installed version; the Debian 11 range would match it.
		{CVEID: "CVE-2025-1111", Ecosystem: "debian", EcosystemRelease: "11", PackageName: "openssl", VersionStartIncluding: "0", VersionEndExcluding: "3.9.9-1"},
		{CVEID: "CVE-2025-1111", Ecosystem: "debian", EcosystemRelease: "12", PackageName: "openssl", VersionStartIncluding: "0", VersionEndExcluding: "3.0.11-1~deb12u1", FixedVersion: "3.0.11-1~deb12u1"},
		// 2222: no fix yet in Debian 12.
		{CVEID: "CVE-2025-2222", Ecosystem: "debian", EcosystemRelease: "12", PackageName: "openssl", VersionStartIncluding: "0"},
		// 3333: only Debian 11 is affected.
		{CVEID: "CVE-2025-3333", Ecosystem: "debian", EcosystemRelease: "11", PackageName: "openssl", VersionStartIncluding: "0", VersionEndExcluding: "9.9.9"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create PackageVulnerability: %v", err)
		}
	}

	run := func(purl, osVersion string) []string {
		t.Helper()
		sbom := &models.SBOM{OSName: "debian", OSVersion: osVersion, Status: "finalized"}
		if err := db.Create(sbom).Error; err != nil {
			t.Fatalf("create SBOM: %v", err)
		}
		c := models.SBOMComponent{SBOMID: sbom.ID, ComponentName: "libssl3", ComponentVersion: "3.0.15-1~deb12u1", ComponentType: "library", PURL: purl}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("create component: %v", err)
		}
		matches, err := NewMatcher(database.NewPostgresManager(db), db).MatchSBOM(context.Background(), sbom, nil)
		if err != nil {
			t.Fatalf("MatchSBOM: %v", err)
		}
		var ids []string
		for _, m := range matches {
			ids = append(ids, m.CVEID)
		}
		sort.Strings(ids)
		return ids
	}

	want := []string{"CVE-2025-2222"}
	// Release from the purl distro qualifier.
	if got := run("pkg:deb/debian/libssl3@3.0.15-1~deb12u1?arch=amd64&distro=debian-12&upstream=openssl", ""); !equalStrings(got, want) {
		t.Errorf("distro qualifier: got %v, want %v", got, want)
	}
	// Release from the SBOM OS version when the purl has none.
	if got := run("pkg:deb/debian/libssl3@3.0.15-1~deb12u1?upstream=openssl", "12"); !equalStrings(got, want) {
		t.Errorf("SBOM OS version: got %v, want %v", got, want)
	}
	// Unknown release: every range applies, as before.
	if got := run("pkg:deb/debian/libssl3@3.0.15-1~deb12u1?upstream=openssl", ""); !equalStrings(got, []string{"CVE-2025-1111", "CVE-2025-2222", "CVE-2025-3333"}) {
		t.Errorf("unknown release: got %v", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Through the OSV mirror tables: before, the two releases' ranges of one advisory were merged by
// advisory id and only one survived, so the result depended on row order.
func TestMatchSBOM_OSVMirrorReleaseRanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.CVE{}, &models.PackageVulnerability{},
		&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	docs := []*osv.Document{
		{ID: "DEBIAN-CVE-2025-1111", Aliases: []string{"CVE-2025-1111"}, Affected: []osv.Affected{
			{Package: osv.Package{Ecosystem: "Debian:11", Name: "openssl"}, Ranges: []osv.Range{{Type: "ECOSYSTEM", Events: []osv.Event{{Introduced: "0"}, {Fixed: "3.9.9-1"}}}}},
			{Package: osv.Package{Ecosystem: "Debian:12", Name: "openssl"}, Ranges: []osv.Range{{Type: "ECOSYSTEM", Events: []osv.Event{{Introduced: "0"}, {Fixed: "3.0.11-1~deb12u1"}}}}},
		}},
		{ID: "DEBIAN-CVE-2025-2222", Aliases: []string{"CVE-2025-2222"}, Affected: []osv.Affected{
			{Package: osv.Package{Ecosystem: "Debian:11", Name: "openssl"}, Ranges: []osv.Range{{Type: "ECOSYSTEM", Events: []osv.Event{{Introduced: "0"}, {Fixed: "1.0"}}}}},
			{Package: osv.Package{Ecosystem: "Debian:12", Name: "openssl"}, Ranges: []osv.Range{{Type: "ECOSYSTEM", Events: []osv.Event{{Introduced: "0"}}}}},
		}},
	}
	for _, d := range docs {
		if _, err := osv.IngestDocument(ctx, db, d); err != nil {
			t.Fatalf("ingest: %v", err)
		}
	}
	sbom := &models.SBOM{OSName: "debian", OSVersion: "12", Status: "finalized"}
	if err := db.Create(sbom).Error; err != nil {
		t.Fatal(err)
	}
	c := models.SBOMComponent{SBOMID: sbom.ID, ComponentName: "libssl3", ComponentVersion: "3.0.15-1~deb12u1", ComponentType: "library",
		PURL: "pkg:deb/debian/libssl3@3.0.15-1~deb12u1?arch=amd64&distro=debian-12&upstream=openssl"}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	matches, err := NewMatcher(database.NewPostgresManager(db), db).MatchSBOM(ctx, sbom, nil)
	if err != nil {
		t.Fatalf("MatchSBOM: %v", err)
	}
	var ids []string
	for _, m := range matches {
		ids = append(ids, m.CVEID)
	}
	sort.Strings(ids)
	if !equalStrings(ids, []string{"CVE-2025-2222"}) {
		t.Fatalf("matches = %v, want only CVE-2025-2222 (no fix in Debian 12)", ids)
	}
}
