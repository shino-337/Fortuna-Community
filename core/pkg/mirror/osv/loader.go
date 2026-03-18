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
	Documents int
	VulnsUpserted int
	PackagesInserted int
	RangesInserted int
}

func normalizeEcosystem(e string) string {
	e = strings.ToLower(strings.TrimSpace(e))
	switch e {
	case "go", "golang":
		return "go"
	default:
		return e
	}
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
// - ecosystem is normalized to lowercase (Go/Golang -> go).
// - aliases are stored as JSON array string (upper-cased IDs).
// - Only SEMVER ranges are ingested (GIT/ECOSYSTEM skipped for Go matching).
func IngestDocument(ctx context.Context, db *gorm.DB, doc *Document) (*Stats, error) {
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
		ID:          doc.ID,
		Summary:     doc.Summary,
		Details:     doc.Details,
		Severity:    "", // optional: computed later
		CVSSScore:   0.0,
		PublishedAt: publishedAt,
		ModifiedAt:  modifiedAt,
		Source:      "osv",
		Aliases:     normalizeAliases(doc.Aliases),
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
			VulnID:      doc.ID,
			Ecosystem:   eco,
			PackageName: name,
		}
		if err := db.WithContext(ctx).
			Where("vuln_id = ? AND ecosystem = ? AND package_name = ?", pkgRow.VulnID, pkgRow.Ecosystem, pkgRow.PackageName).
			FirstOrCreate(&pkgRow).Error; err != nil {
			return nil, fmt.Errorf("ingest OSV package: %w", err)
		}
		stats.PackagesInserted++

		for _, r := range aff.Ranges {
			if strings.ToUpper(strings.TrimSpace(r.Type)) != "SEMVER" {
				continue
			}
			// Flatten event pairs. We support sequences like:
			// introduced -> fixed (or last_affected), repeated.
			var currentIntroduced string
			for _, ev := range r.Events {
				if ev.Introduced != "" {
					currentIntroduced = strings.TrimSpace(ev.Introduced)
					continue
				}
				if ev.Fixed != "" || ev.LastAffected != "" {
					row := models.OSVRange{
						PackageID:    pkgRow.ID,
						RangeType:    "SEMVER",
						Introduced:   strings.TrimSpace(currentIntroduced),
						Fixed:        strings.TrimSpace(ev.Fixed),
						LastAffected: strings.TrimSpace(ev.LastAffected),
					}
					if err := db.WithContext(ctx).Create(&row).Error; err != nil {
						return nil, fmt.Errorf("ingest OSV range: %w", err)
					}
					stats.RangesInserted++
				}
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
		if _, err := IngestDocument(ctx, db, doc); err != nil {
			return documents, fmt.Errorf("ingest %q: %w", path, err)
		}
		documents++
	}
	return documents, nil
}

