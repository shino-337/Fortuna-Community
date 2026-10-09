package database

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/cve/catalogtest"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openCatalogDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1) // one in-memory database
	return db
}

const critical31 = "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H" // 9.8

// A Go advisory is reported under its CVE alias, with the advisory's own rating and its range.
func TestVersionedCatalogQuery_GoModule(t *testing.T) {
	db := openCatalogDB(t)
	catalogtest.Seed(t, db, catalogtest.Advisory{
		ID: "GO-TEST-1", Aliases: []string{"CVE-2023-1234"}, Summary: "logrus vuln", Details: "details", Severity: "HIGH",
		Affected: []catalogtest.Range{{Ecosystem: "Go", Package: "github.com/sirupsen/logrus", Type: "SEMVER", Fixed: "v1.9.1"}},
	})

	got, err := NewPostgresManager(db).GetVulnerabilitiesForPackages(context.Background(), "go", []string{"github.com/sirupsen/logrus", "other"})
	if err != nil {
		t.Fatalf("GetVulnerabilitiesForPackages: %v", err)
	}
	if len(got["other"]) != 0 {
		t.Fatalf("unrelated package matched: %+v", got["other"])
	}
	cves := got["github.com/sirupsen/logrus"]
	if len(cves) != 1 {
		t.Fatalf("expected 1 CVE, got %d (%+v)", len(cves), cves)
	}
	c := cves[0]
	if c.ID != "CVE-2023-1234" || c.AdvisoryID != "GO-TEST-1" {
		t.Fatalf("id = %q advisory = %q", c.ID, c.AdvisoryID)
	}
	if c.Constraint != "<v1.9.1" || c.FixedVersion != "v1.9.1" {
		t.Fatalf("constraint = %q fixed = %q", c.Constraint, c.FixedVersion)
	}
	if c.Severity != "HIGH" || c.SeveritySource != "vendor" {
		t.Fatalf("severity = %q from %q", c.Severity, c.SeveritySource)
	}
	if c.Description != "details" {
		t.Fatalf("description = %q", c.Description)
	}
}

// Without its own rating or score, a range takes the NVD score of its CVE, then the best score
// of another advisory about that CVE alone.
func TestVersionedCatalogQuery_SeverityFallbacks(t *testing.T) {
	db := openCatalogDB(t)
	catalogtest.Seed(t, db,
		catalogtest.Advisory{ID: "ALPINE-CVE-2023-42363", Aliases: []string{"CVE-2023-42363"},
			Affected: []catalogtest.Range{{Ecosystem: "Alpine:v3.20", Package: "busybox", Fixed: "1.36.1-r16"}}},
		catalogtest.Advisory{ID: "ALPINE-CVE-2024-0001", Aliases: []string{"CVE-2024-0001"},
			Affected: []catalogtest.Range{{Ecosystem: "Alpine:v3.20", Package: "busybox", Fixed: "1.36.1-r20"}}},
		// Two other advisories about CVE-2024-0001; the best score wins.
		catalogtest.Advisory{ID: "GHSA-aaaa-bbbb-cccc", Aliases: []string{"CVE-2024-0001"}, CVSSv3: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:L/I:N/A:N"},
		catalogtest.Advisory{ID: "UBUNTU-CVE-2024-0001", Upstream: []string{"CVE-2024-0001"}, CVSSv3: critical31},
	)
	catalogtest.NVD(t, db, "CVE-2023-42363", 5.5, "CVSS:3.1/AV:L/AC:L/PR:N/UI:R/S:U/C:N/I:N/A:H")

	got, err := NewPostgresManager(db).GetVulnerabilitiesForPackages(context.Background(), "alpine", []string{"busybox"})
	if err != nil {
		t.Fatalf("GetVulnerabilitiesForPackages: %v", err)
	}
	byID := map[string]string{}
	for _, c := range got["busybox"] {
		if c.Release != "3.20" {
			t.Errorf("%s release = %q", c.ID, c.Release)
		}
		byID[c.ID] = c.SeveritySource + " " + c.Severity
	}
	want := map[string]string{"CVE-2023-42363": "nvd MEDIUM", "CVE-2024-0001": "cve_cvss CRITICAL"}
	if len(byID) != len(want) || byID["CVE-2023-42363"] != want["CVE-2023-42363"] || byID["CVE-2024-0001"] != want["CVE-2024-0001"] {
		t.Fatalf("severities = %v, want %v", byID, want)
	}
}

// Without a loaded catalog every package gets no vulnerabilities and no error.
func TestVersionedCatalogQuery_NoCatalog(t *testing.T) {
	for name, db := range map[string]*gorm.DB{"no tables": openCatalogDB(t), "empty catalog": openCatalogDB(t)} {
		if name == "empty catalog" {
			catalogtest.ActiveGeneration(t, db)
		}
		got, err := NewPostgresManager(db).GetVulnerabilitiesForPackages(context.Background(), "debian", []string{"openssl"})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cves, ok := got["openssl"]; !ok || len(cves) != 0 {
			t.Fatalf("%s: got %v", name, got)
		}
	}
}
