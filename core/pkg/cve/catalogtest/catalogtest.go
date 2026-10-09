// Package catalogtest seeds the versioned vulnerability catalog in tests. Advisories are given
// in OSV terms and converted by the loader, so tests read what a real load would have written.
package catalogtest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/cve/loader"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Advisory is one OSV advisory.
type Advisory struct {
	ID        string
	Aliases   []string
	Upstream  []string
	Related   []string
	Summary   string
	Details   string
	Published time.Time
	Severity  string // the source's own rating (GitHub database_specific.severity)
	CVSSv3    string // CVSS v3 vector
	Withdrawn bool
	Affected  []Range
}

// Range is one affected package of an advisory with one affected interval.
type Range struct {
	Ecosystem    string // OSV ecosystem: "Debian:12", "Go", "npm", …
	Package      string
	Type         string // ECOSYSTEM (default) or SEMVER
	Introduced   string // "0" by default
	Fixed        string
	LastAffected string
	Versions     []string // explicit versions; used alone when Type is "EXPLICIT"
	Urgency      string   // Debian urgency of the package
}

// Install creates the versioned catalog tables and catalog_generations when they are missing.
func Install(t testing.TB, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&models.CatalogGeneration{}); err != nil {
		t.Fatalf("catalogtest: catalog_generations: %v", err)
	}
	textArray, json, serial := "TEXT", "TEXT", "INTEGER PRIMARY KEY AUTOINCREMENT"
	if db.Dialector.Name() == "postgres" {
		textArray, json, serial = "TEXT[]", "JSONB", "BIGSERIAL PRIMARY KEY"
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS vuln_advisories (
			advisory_id VARCHAR(255) NOT NULL, valid_from_gen BIGINT NOT NULL, valid_to_gen BIGINT,
			source VARCHAR(32) NOT NULL DEFAULT '', kind VARCHAR(16) NOT NULL DEFAULT 'vulnerability',
			summary TEXT NOT NULL DEFAULT '', details TEXT NOT NULL DEFAULT '',
			published_at TIMESTAMP, modified_at TIMESTAMP,
			cvss_v3_vector TEXT NOT NULL DEFAULT '', cvss_v3_score NUMERIC(3,1),
			cvss_v4_vector TEXT NOT NULL DEFAULT '', cvss_v4_score NUMERIC(3,1),
			source_severity VARCHAR(16) NOT NULL DEFAULT '', cwe_ids ` + textArray + `,
			reference_urls ` + json + `, content_sha256 CHAR(64) NOT NULL,
			PRIMARY KEY (advisory_id, valid_from_gen))`,
		`CREATE TABLE IF NOT EXISTS vuln_advisory_refs (
			advisory_id VARCHAR(255) NOT NULL, ref_id VARCHAR(255) NOT NULL, relation VARCHAR(16) NOT NULL,
			ref_kind VARCHAR(16) NOT NULL, valid_from_gen BIGINT NOT NULL, valid_to_gen BIGINT,
			PRIMARY KEY (advisory_id, ref_id, valid_from_gen))`,
		`CREATE TABLE IF NOT EXISTS vuln_affected (
			id ` + serial + `, advisory_id VARCHAR(255) NOT NULL, ecosystem VARCHAR(64) NOT NULL,
			release VARCHAR(64) NOT NULL DEFAULT '', package_name VARCHAR(512) NOT NULL,
			range_type VARCHAR(16) NOT NULL, introduced TEXT NOT NULL DEFAULT '', fixed TEXT NOT NULL DEFAULT '',
			last_affected TEXT NOT NULL DEFAULT '', vendor_severity VARCHAR(16) NOT NULL DEFAULT '',
			valid_from_gen BIGINT NOT NULL, valid_to_gen BIGINT)`,
		`CREATE TABLE IF NOT EXISTS vulnerabilities (
			vuln_id VARCHAR(255) PRIMARY KEY, title TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '',
			published_at TIMESTAMP, nvd_cvss_v3_vector TEXT NOT NULL DEFAULT '', nvd_cvss_v3_score NUMERIC(3,1),
			nvd_cvss_v4_vector TEXT NOT NULL DEFAULT '', nvd_cvss_v4_score NUMERIC(3,1), cwe_ids ` + textArray + `,
			kev_added_at DATE, kev_due_date DATE, kev_ransomware BOOLEAN, epss_score REAL, epss_percentile REAL,
			epss_date DATE, nvd_updated_at TIMESTAMP, kev_updated_at TIMESTAMP, epss_updated_at TIMESTAMP)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("catalogtest: %v", err)
		}
	}
}

// ActiveGeneration returns the active cve generation, creating one when there is none.
func ActiveGeneration(t testing.TB, db *gorm.DB) uint {
	t.Helper()
	Install(t, db)
	gen, err := loader.ActiveCVEGeneration(t.Context(), db)
	if err != nil {
		t.Fatalf("catalogtest: active generation: %v", err)
	}
	if gen != nil {
		return gen.ID
	}
	return NewGeneration(t, db)
}

// NewGeneration activates a new cve generation that sees every advisory seeded so far.
func NewGeneration(t testing.TB, db *gorm.DB) uint {
	t.Helper()
	Install(t, db)
	now := time.Now()
	g := models.CatalogGeneration{CatalogType: "cve", SourceName: "osv", Status: "active",
		StartedAt: now, ActivatedAt: &now, RecordCounts: "{}"}
	if err := db.Create(&g).Error; err != nil {
		t.Fatalf("catalogtest: create generation: %v", err)
	}
	return g.ID
}

// Seed writes advisories into the active generation (created when missing) and returns it.
func Seed(t testing.TB, db *gorm.DB, advs ...Advisory) uint {
	t.Helper()
	gen := ActiveGeneration(t, db)
	parsed := make([]*loader.ParsedAdvisory, 0, len(advs))
	for _, a := range advs {
		parsed = append(parsed, Parse(t, a))
	}
	Write(t, db, gen, parsed...)
	return gen
}

// Parse converts a to the advisory the loader would store.
func Parse(t testing.TB, a Advisory) *loader.ParsedAdvisory {
	t.Helper()
	raw, err := json.Marshal(a.osv())
	if err != nil {
		t.Fatalf("catalogtest: %v", err)
	}
	doc, err := loader.ParseBytes(raw, a.ID)
	if err != nil {
		t.Fatalf("catalogtest: %v", err)
	}
	return loader.BuildAdvisory(doc, raw)
}

func (a Advisory) osv() loader.OSVVulnerability {
	doc := loader.OSVVulnerability{
		ID: a.ID, Summary: a.Summary, Details: a.Details,
		Aliases: a.Aliases, Upstream: a.Upstream, Related: a.Related,
	}
	if !a.Published.IsZero() {
		doc.Published = a.Published.UTC().Format(time.RFC3339)
	}
	if a.Withdrawn {
		doc.Withdrawn = "2026-01-01T00:00:00Z"
	}
	if a.Severity != "" {
		doc.DatabaseSpecific = map[string]interface{}{"severity": a.Severity}
	}
	if a.CVSSv3 != "" {
		doc.Severity = []loader.OSVSeverity{{Type: "CVSS_V3", Score: a.CVSSv3}}
	}
	for _, r := range a.Affected {
		aff := loader.OSVAffected{Package: loader.OSVPackage{Ecosystem: r.Ecosystem, Name: r.Package}, Versions: r.Versions}
		if r.Urgency != "" {
			aff.EcosystemSpecific = map[string]interface{}{"urgency": r.Urgency}
		}
		if r.Type != "EXPLICIT" {
			typ := r.Type
			if typ == "" {
				typ = "ECOSYSTEM"
			}
			introduced := r.Introduced
			if introduced == "" {
				introduced = "0"
			}
			events := []loader.OSVEvent{{Introduced: introduced}}
			switch {
			case r.Fixed != "":
				events = append(events, loader.OSVEvent{Fixed: r.Fixed})
			case r.LastAffected != "":
				events = append(events, loader.OSVEvent{LastAffected: r.LastAffected})
			}
			aff.Ranges = []loader.OSVRange{{Type: typ, Events: events}}
		}
		doc.Affected = append(doc.Affected, aff)
	}
	return doc
}

// Write stores parsed advisories as current versions loaded by generation gen.
func Write(t testing.TB, db *gorm.DB, gen uint, advs ...*loader.ParsedAdvisory) {
	t.Helper()
	Install(t, db)
	if db.Dialector.Name() == "postgres" {
		if _, err := loader.NewVersionedCatalog(db, gen).Write(t.Context(), advs); err != nil {
			t.Fatalf("catalogtest: write advisories: %v", err)
		}
		return
	}
	for _, a := range advs {
		if a.Withdrawn {
			continue
		}
		sha := a.ContentSHA256
		if sha == "" {
			sha = fmt.Sprintf("%x", sha256.Sum256([]byte(a.ID)))
		}
		cwes := "{" + strings.Join(a.CWEIDs, ",") + "}"
		refs := a.References
		if refs == "" {
			refs = "[]"
		}
		if err := db.Exec(`INSERT INTO vuln_advisories (advisory_id, valid_from_gen, source, kind, summary, details,
			published_at, modified_at, cvss_v3_vector, cvss_v3_score, cvss_v4_vector, cvss_v4_score,
			source_severity, cwe_ids, reference_urls, content_sha256) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			a.ID, gen, a.Source, a.Kind, a.Summary, a.Details, a.PublishedAt, a.ModifiedAt,
			a.CVSSv3Vector, a.CVSSv3Score, a.CVSSv4Vector, a.CVSSv4Score, a.SourceSeverity, cwes, refs, sha).Error; err != nil {
			t.Fatalf("catalogtest: insert advisory %s: %v", a.ID, err)
		}
		for _, r := range a.Refs {
			if err := db.Exec(`INSERT INTO vuln_advisory_refs (advisory_id, ref_id, relation, ref_kind, valid_from_gen)
				VALUES (?, ?, ?, ?, ?)`, a.ID, r.RefID, r.Relation, r.RefKind, gen).Error; err != nil {
				t.Fatalf("catalogtest: insert ref of %s: %v", a.ID, err)
			}
		}
		for _, r := range a.Affected {
			if err := db.Exec(`INSERT INTO vuln_affected (advisory_id, ecosystem, release, package_name, range_type,
				introduced, fixed, last_affected, vendor_severity, valid_from_gen) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				a.ID, r.Ecosystem, r.Release, r.PackageName, r.RangeType, r.Introduced, r.Fixed, r.LastAffected,
				r.VendorSeverity, gen).Error; err != nil {
				t.Fatalf("catalogtest: insert range of %s: %v", a.ID, err)
			}
		}
	}
}

// NVD stores the NVD CVSS v3 score of a CVE in the vulnerabilities table.
func NVD(t testing.TB, db *gorm.DB, cveID string, score float64, vector string) {
	t.Helper()
	Install(t, db)
	if err := db.Exec(`INSERT INTO vulnerabilities (vuln_id, nvd_cvss_v3_score, nvd_cvss_v3_vector, cwe_ids) VALUES (?, ?, ?, '{}')`,
		cveID, score, vector).Error; err != nil {
		t.Fatalf("catalogtest: insert NVD score of %s: %v", cveID, err)
	}
}
