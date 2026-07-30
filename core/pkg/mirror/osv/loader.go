package osv

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

type Stats struct {
	Documents        int
	VulnsUpserted    int
	PackagesInserted int
	RangesInserted   int
}

func normalizeEcosystem(e string) string {
	e = strings.ToLower(strings.TrimSpace(e))
	// Collapse versioned OSV distro keys (Debian:10, Alpine:v3.19, …) like pkg/cve/loader.
	if strings.HasPrefix(e, "debian:") || e == "debian" {
		return "debian"
	}
	if strings.HasPrefix(e, "ubuntu:") || e == "ubuntu" {
		return "ubuntu"
	}
	if strings.HasPrefix(e, "alpine:") || e == "alpine" {
		return "alpine"
	}
	if e == "red hat" || strings.HasPrefix(e, "red hat:") {
		return "redhat"
	}
	if e == "rhel" || strings.HasPrefix(e, "rhel:") {
		return "redhat"
	}
	switch e {
	case "go", "golang":
		return "go"
	case "pypi":
		return "pypi"
	case "npm":
		return "npm"
	case "maven":
		return "maven"
	case "nuget":
		return "nuget"
	case "crates.io", "cargo":
		return "cargo"
	case "rubygems":
		return "rubygems"
	default:
		return e
	}
}

// ingestRangeEvents flattens OSV range events into osv_ranges rows.
// storeType is SEMVER or ECOSYSTEM (stored verbatim for matcher policy).
func ingestRangeEvents(ctx context.Context, db *gorm.DB, pkgRow *models.OSVPackage, storeType string, r Range, stats *Stats, catalogGenerationID uint) error {
	var currentIntroduced string
	for _, ev := range r.Events {
		if ev.Introduced != "" {
			currentIntroduced = strings.TrimSpace(ev.Introduced)
			continue
		}
		if ev.Fixed != "" || ev.LastAffected != "" {
			row := models.OSVRange{
				PackageID:           pkgRow.ID,
				RangeType:           storeType,
				Introduced:          strings.TrimSpace(currentIntroduced),
				Fixed:               strings.TrimSpace(ev.Fixed),
				LastAffected:        strings.TrimSpace(ev.LastAffected),
				CatalogGenerationID: catalogGenerationID,
			}
			if err := db.WithContext(ctx).Create(&row).Error; err != nil {
				return fmt.Errorf("ingest OSV range: %w", err)
			}
			stats.RangesInserted++
		}
	}
	return nil
}

func normalizeAliases(aliases []string) string {
	if len(aliases) == 0 {
		return "[]"
	}
	out := make([]string, 0, len(aliases))
	seen := map[string]bool{}
	for _, a := range aliases {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		upper := strings.ToUpper(a)
		if seen[upper] {
			continue
		}
		seen[upper] = true
		out = append(out, upper)
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// IngestDocument flattens a single OSV JSON document into mirror tables:
// osv_vulnerabilities, osv_packages, osv_ranges.
//
// Notes:
// - ecosystem is normalized (Go/Golang -> go; Debian:10 -> debian; Red Hat -> redhat).
// - aliases are stored as JSON array string (upper-cased IDs).
// - SEMVER ranges are ingested for all ecosystems.
// - ECOSYSTEM ranges are ingested for non-go ecosystems (Alpine/Debian/RPM feeds); skipped for go (modules use SEMVER).
func IngestDocument(ctx context.Context, db *gorm.DB, doc *Document) (*Stats, error) {
	return IngestDocumentWithGeneration(ctx, db, doc, 0)
}

func IngestDocumentWithGeneration(ctx context.Context, db *gorm.DB, doc *Document, catalogGenerationID uint) (*Stats, error) {
	if db == nil || doc == nil {
		return nil, fmt.Errorf("ingest OSV: db/doc required")
	}
	stats := &Stats{Documents: 1}

	var publishedAt time.Time
	if doc.Published != "" {
		if t, err := time.Parse(time.RFC3339, doc.Published); err == nil {
			publishedAt = t
		}
	}
	var modifiedAt time.Time
	if doc.Modified != "" {
		if t, err := time.Parse(time.RFC3339, doc.Modified); err == nil {
			modifiedAt = t
		}
	}

	v := models.OSVVulnerability{
		ID:                  doc.ID,
		Summary:             doc.Summary,
		Details:             doc.Details,
		Severity:            "", // optional: computed later
		CVSSScore:           0.0,
		PublishedAt:         publishedAt,
		ModifiedAt:          modifiedAt,
		Source:              "osv",
		Aliases:             normalizeAliases(doc.Aliases),
		CatalogGenerationID: catalogGenerationID,
	}

	if err := db.WithContext(ctx).
		Where("id = ?", v.ID).
		Assign(v).
		FirstOrCreate(&v).Error; err != nil {
		return nil, fmt.Errorf("ingest OSV vulnerability: %w", err)
	}
	stats.VulnsUpserted++

	for _, aff := range doc.Affected {
		eco := normalizeEcosystem(aff.Package.Ecosystem)
		name := strings.TrimSpace(aff.Package.Name)
		if eco == "" || name == "" {
			continue
		}

		pkgRow := models.OSVPackage{
			VulnID:              doc.ID,
			Ecosystem:           eco,
			PackageName:         name,
			CatalogGenerationID: catalogGenerationID,
		}
		if err := db.WithContext(ctx).
			Where("vuln_id = ? AND ecosystem = ? AND package_name = ?", pkgRow.VulnID, pkgRow.Ecosystem, pkgRow.PackageName).
			Assign(map[string]interface{}{"catalog_generation_id": catalogGenerationID}).
			FirstOrCreate(&pkgRow).Error; err != nil {
			return nil, fmt.Errorf("ingest OSV package: %w", err)
		}
		stats.PackagesInserted++

		for _, r := range aff.Ranges {
			rt := strings.ToUpper(strings.TrimSpace(r.Type))
			switch rt {
			case "SEMVER":
				if err := ingestRangeEvents(ctx, db, &pkgRow, "SEMVER", r, stats, catalogGenerationID); err != nil {
					return nil, err
				}
			case "ECOSYSTEM":
				if eco == "go" {
					continue
				}
				if err := ingestRangeEvents(ctx, db, &pkgRow, "ECOSYSTEM", r, stats, catalogGenerationID); err != nil {
					return nil, err
				}
			default:
				continue
			}
		}
	}

	return stats, nil
}

// LoadFromDir reads all *.json files in dir and ingests each as an OSV document into the mirror tables.
// Used by UpdateDatabase when FORTUNA_OSV_SOURCE_DIR is set.
func LoadFromDir(ctx context.Context, db *gorm.DB, dir string) (documents int, err error) {
	if db == nil || dir == "" {
		return 0, fmt.Errorf("db and dir required")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read dir %q: %w", dir, err)
	}
	catalogGenerationID := activeCatalogGenerationID(ctx, db)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return documents, fmt.Errorf("read %q: %w", path, err)
		}
		doc, err := ParseDocument(data)
		if err != nil {
			return documents, fmt.Errorf("parse %q: %w", path, err)
		}
		if _, err := IngestDocumentWithGeneration(ctx, db, doc, catalogGenerationID); err != nil {
			return documents, fmt.Errorf("ingest %q: %w", path, err)
		}
		documents++
	}
	return documents, nil
}

func activeCatalogGenerationID(ctx context.Context, db *gorm.DB) uint {
	if db == nil || !db.Migrator().HasTable("catalog_generations") {
		return 0
	}
	var generation models.CatalogGeneration
	if err := db.WithContext(ctx).
		Where("catalog_type = ? AND status = ?", "cve", "active").
		Order("activated_at DESC, id DESC").
		First(&generation).Error; err != nil {
		return 0
	}
	return generation.ID
}
