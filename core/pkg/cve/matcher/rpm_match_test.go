package matcher

import (
	"context"
	"sort"
	"testing"

	"github.com/fortuna/core/pkg/cve/catalogtest"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Syft rpm purls use the os-release ID as namespace (almalinux, rocky, redhat) and carry the
// source rpm; OSV stores AlmaLinux as "alma", names Rocky advisories by source package and
// scopes Red Hat streams to a RHEL major.
func TestMatchSBOM_RPMDistros(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	catalogtest.Seed(t, db,
		catalogtest.Advisory{ID: "CVE-2026-0001", Severity: "HIGH", Affected: []catalogtest.Range{
			{Ecosystem: "AlmaLinux:8", Package: "vim-minimal", Fixed: "2:8.0.1763-32.el8_10"},
		}},
		catalogtest.Advisory{ID: "CVE-2026-0002", Severity: "HIGH", Affected: []catalogtest.Range{
			{Ecosystem: "Rocky Linux:8", Package: "vim", Fixed: "2:8.0.1763-32.el8_10"},
		}},
		catalogtest.Advisory{ID: "CVE-2026-0003", Severity: "HIGH", Affected: []catalogtest.Range{
			{Ecosystem: "Red Hat:enterprise_linux:9::baseos", Package: "openssl-libs", Fixed: "1:3.0.7-28.el9_4"},
		}},
		// RHEL 8 stream only: must not match a RHEL 9 package.
		catalogtest.Advisory{ID: "CVE-2026-0004", Severity: "HIGH", Affected: []catalogtest.Range{
			{Ecosystem: "Red Hat:enterprise_linux:8::baseos", Package: "openssl-libs", Fixed: "1:9.9.9-1.el8"},
		}},
	)
	run := func(osName, name, version, purl string) []string {
		t.Helper()
		sbom := &models.SBOM{OSName: osName, Status: "finalized"}
		if err := db.Create(sbom).Error; err != nil {
			t.Fatal(err)
		}
		c := models.SBOMComponent{SBOMID: sbom.ID, ComponentName: name, ComponentVersion: version, ComponentType: "library", PURL: purl}
		if err := db.Create(&c).Error; err != nil {
			t.Fatal(err)
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
	cases := []struct {
		name, osName, pkg, version, purl string
		want                             []string
	}{
		{"alma namespace", "almalinux", "vim-minimal", "2:8.0.1763-30.el8_10",
			"pkg:rpm/almalinux/vim-minimal@8.0.1763-30.el8_10?arch=x86_64&distro=almalinux-8.10&epoch=2&upstream=vim-8.0.1763-30.el8_10.src.rpm", []string{"CVE-2026-0001"}},
		{"alma fixed", "almalinux", "vim-minimal", "2:8.0.1763-32.el8_10",
			"pkg:rpm/almalinux/vim-minimal@8.0.1763-32.el8_10?arch=x86_64&distro=almalinux-8.10&epoch=2&upstream=vim-8.0.1763-32.el8_10.src.rpm", nil},
		{"rocky by source rpm", "rocky", "vim-minimal", "2:8.0.1763-30.el8_10",
			"pkg:rpm/rocky/vim-minimal@8.0.1763-30.el8_10?arch=x86_64&distro=rocky-8.10&epoch=2&upstream=vim-8.0.1763-30.el8_10.src.rpm", []string{"CVE-2026-0002"}},
		{"rhel release 9 only", "rhel", "openssl-libs", "1:3.0.7-27.el9",
			"pkg:rpm/redhat/openssl-libs@3.0.7-27.el9?arch=x86_64&distro=rhel-9.4&epoch=1&upstream=openssl-3.0.7-27.el9.src.rpm", []string{"CVE-2026-0003"}},
	}
	for _, tc := range cases {
		if got := run(tc.osName, tc.pkg, tc.version, tc.purl); !equalStrings(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRPMSourceName(t *testing.T) {
	for in, want := range map[string]string{
		"vim-8.0.1763-32.el8_10.src.rpm":     "vim",
		"openssl-3.0.7-27.el9.src.rpm":       "openssl",
		"python3.11-3.11.9-7.el9.src.rpm":    "python3.11",
		"kernel-rt-5.14.0-427.el9.nosrc.rpm": "kernel-rt",
		"broken.src.rpm":                     "",
	} {
		if got := rpmSourceName(in); got != want {
			t.Errorf("rpmSourceName(%q) = %q, want %q", in, got, want)
		}
	}
}
