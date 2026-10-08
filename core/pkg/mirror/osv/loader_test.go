package osv

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestIngestDocument_FlattensPackagesAndRanges(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	doc := &Document{
		ID:      "GO-TEST-1234",
		Summary: "test",
		Details: "details",
		Aliases: []string{"cve-2023-1234", " GHSA-aaaa-bbbb-cccc "},
		Affected: []Affected{
			{
				Package: Package{Ecosystem: "Go", Name: "github.com/sirupsen/logrus"},
				Ranges: []Range{
					{
						Type: "SEMVER",
						Events: []Event{
							{Introduced: "0"},
							{Fixed: "v1.9.1"},
							{Introduced: "v1.10.0"},
							{LastAffected: "v1.10.2"},
						},
					},
				},
			},
			{
				Package: Package{Ecosystem: "Golang", Name: "github.com/sirupsen/logrus/hooks/syslog"},
				Ranges: []Range{
					{
						Type: "ECOSYSTEM",
						Events: []Event{
							{Introduced: "0"},
							{Fixed: "v9.9.9"},
						},
					},
				},
			},
		},
	}

	stats, err := IngestDocument(context.Background(), db, doc)
	if err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if stats.VulnsUpserted != 1 {
		t.Fatalf("vulns upserted=%d, want 1", stats.VulnsUpserted)
	}
	// 2 packages inserted; second is still go — ECOSYSTEM ranges for go are skipped; package row is still inserted
	if stats.PackagesInserted != 2 {
		t.Fatalf("packages inserted=%d, want 2", stats.PackagesInserted)
	}
	// 2 SEMVER intervals from first affected only
	if stats.RangesInserted != 2 {
		t.Fatalf("ranges inserted=%d, want 2", stats.RangesInserted)
	}

	var v models.OSVVulnerability
	if err := db.First(&v, "id = ?", "GO-TEST-1234").Error; err != nil {
		t.Fatalf("load vuln: %v", err)
	}
	if v.Aliases == "" || v.Aliases == "[]" {
		t.Fatalf("expected aliases JSON to be stored, got %q", v.Aliases)
	}

	var pkgs []models.OSVPackage
	if err := db.Find(&pkgs).Error; err != nil {
		t.Fatalf("load packages: %v", err)
	}
	// verify ecosystem normalized to lowercase go
	for _, p := range pkgs {
		if p.Ecosystem != "go" {
			t.Fatalf("expected ecosystem normalized to go, got %q", p.Ecosystem)
		}
	}
}

func TestIngestDocument_AlpineECOSYSTEMRange(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	doc := &Document{
		ID: "ALPINE-TEST-1", Summary: "s", Details: "d",
		Affected: []Affected{
			{
				Package: Package{Ecosystem: "Alpine:v3.19", Name: "busybox"},
				Ranges: []Range{
					{Type: "ECOSYSTEM", Events: []Event{{Introduced: "0"}, {Fixed: "1.99.0"}}},
				},
			},
		},
	}
	stats, err := IngestDocument(context.Background(), db, doc)
	if err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}
	if stats.RangesInserted != 1 {
		t.Fatalf("ranges inserted=%d, want 1", stats.RangesInserted)
	}
	var r models.OSVRange
	if err := db.First(&r).Error; err != nil {
		t.Fatalf("range: %v", err)
	}
	if r.RangeType != "ECOSYSTEM" {
		t.Fatalf("range type=%q, want ECOSYSTEM", r.RangeType)
	}
	var p models.OSVPackage
	if err := db.First(&p).Error; err != nil {
		t.Fatalf("pkg: %v", err)
	}
	if p.Ecosystem != "alpine" {
		t.Fatalf("ecosystem=%q, want alpine", p.Ecosystem)
	}
}

func TestIngestDocument_KeepsReleasesAndHandlesWithdrawal(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.OSVVulnerability{}, &models.OSVPackage{}, &models.OSVRange{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	doc := &Document{
		ID: "DEBIAN-CVE-2025-0001",
		Affected: []Affected{
			{Package: Package{Ecosystem: "Debian:11", Name: "openssl"}, Ranges: []Range{{Type: "ECOSYSTEM", Events: []Event{{Introduced: "0"}, {Fixed: "1.1.1w-0+deb11u2"}}}}},
			{Package: Package{Ecosystem: "Debian:12", Name: "openssl"}, Ranges: []Range{{Type: "ECOSYSTEM", Events: []Event{{Introduced: "0"}}}}},
			{Package: Package{Ecosystem: "PyPI", Name: "demo"}, Versions: []string{"1.0", "1.1"}},
		},
	}
	ctx := context.Background()
	// Ingesting twice must not duplicate ranges.
	for i := 0; i < 2; i++ {
		if _, err := IngestDocument(ctx, db, doc); err != nil {
			t.Fatalf("IngestDocument: %v", err)
		}
	}

	var pkgs []models.OSVPackage
	db.Order("ecosystem, ecosystem_release").Find(&pkgs)
	if len(pkgs) != 3 || pkgs[0].EcosystemRelease != "11" || pkgs[1].EcosystemRelease != "12" || pkgs[2].Ecosystem != "pypi" {
		t.Fatalf("packages = %+v", pkgs)
	}
	var ranges []models.OSVRange
	db.Order("id").Find(&ranges)
	if len(ranges) != 4 {
		t.Fatalf("ranges = %+v, want 4 (deb11 fixed, deb12 open, two pypi versions)", ranges)
	}
	var open, versions int
	for _, r := range ranges {
		if r.RangeType == "ECOSYSTEM" && r.Fixed == "" && r.LastAffected == "" {
			open++
		}
		if r.RangeType == "VERSION" && r.Introduced == r.LastAffected {
			versions++
		}
	}
	if open != 1 || versions != 2 {
		t.Fatalf("open=%d versions=%d, ranges=%+v", open, versions, ranges)
	}

	doc.Withdrawn = "2025-06-01T00:00:00Z"
	stats, err := IngestDocument(ctx, db, doc)
	if err != nil {
		t.Fatalf("IngestDocument withdrawn: %v", err)
	}
	var nVuln, nPkg, nRange int64
	db.Model(&models.OSVVulnerability{}).Count(&nVuln)
	db.Model(&models.OSVPackage{}).Count(&nPkg)
	db.Model(&models.OSVRange{}).Count(&nRange)
	if stats.VulnsWithdrawn != 1 || nVuln != 0 || nPkg != 0 || nRange != 0 {
		t.Fatalf("withdrawn advisory left rows: vulns=%d packages=%d ranges=%d", nVuln, nPkg, nRange)
	}
}
