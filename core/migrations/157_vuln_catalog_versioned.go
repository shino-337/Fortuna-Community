package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration157_VulnCatalogVersioned creates the versioned vulnerability catalog.
//
// vuln_advisories, vuln_advisory_refs and vuln_affected keep one row per advisory version:
// valid_from_gen is the catalog generation that loaded it and valid_to_gen the generation that
// replaced or removed it (NULL while current). A generation reads the rows with
// valid_from_gen <= g AND (valid_to_gen IS NULL OR valid_to_gen > g), so each load writes only
// the advisories that changed.
//
// vulnerabilities holds one row per CVE (or per advisory without a CVE) for KEV, EPSS and NVD
// enrichment, and vuln_feed_state the download state of each feed. The legacy cves and
// package_vulnerabilities tables stay until the matcher reads the new ones.
func Migration157_VulnCatalogVersioned(db *gorm.DB) error {
	log.Println("[Migration 157] Creating versioned vulnerability catalog tables...")

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS vuln_advisories (
			advisory_id     VARCHAR(255) NOT NULL,
			valid_from_gen  BIGINT NOT NULL,
			valid_to_gen    BIGINT,
			source          VARCHAR(32) NOT NULL DEFAULT '',
			kind            VARCHAR(16) NOT NULL DEFAULT 'vulnerability',
			summary         TEXT NOT NULL DEFAULT '',
			details         TEXT NOT NULL DEFAULT '',
			published_at    TIMESTAMPTZ,
			modified_at     TIMESTAMPTZ,
			cvss_v3_vector  TEXT NOT NULL DEFAULT '',
			cvss_v3_score   NUMERIC(3,1),
			cvss_v4_vector  TEXT NOT NULL DEFAULT '',
			cvss_v4_score   NUMERIC(3,1),
			source_severity VARCHAR(16) NOT NULL DEFAULT '',
			cwe_ids         TEXT[] NOT NULL DEFAULT '{}',
			reference_urls  JSONB NOT NULL DEFAULT '[]',
			content_sha256  CHAR(64) NOT NULL,
			PRIMARY KEY (advisory_id, valid_from_gen)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_vuln_advisories_current ON vuln_advisories (advisory_id) WHERE valid_to_gen IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_advisories_from ON vuln_advisories (valid_from_gen)`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_advisories_to ON vuln_advisories (valid_to_gen) WHERE valid_to_gen IS NOT NULL`,

		`CREATE TABLE IF NOT EXISTS vuln_advisory_refs (
			advisory_id    VARCHAR(255) NOT NULL,
			ref_id         VARCHAR(255) NOT NULL,
			relation       VARCHAR(16) NOT NULL,
			ref_kind       VARCHAR(16) NOT NULL,
			valid_from_gen BIGINT NOT NULL,
			valid_to_gen   BIGINT,
			PRIMARY KEY (advisory_id, ref_id, valid_from_gen)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_advisory_refs_ref ON vuln_advisory_refs (ref_id)`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_advisory_refs_from ON vuln_advisory_refs (valid_from_gen)`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_advisory_refs_to ON vuln_advisory_refs (valid_to_gen) WHERE valid_to_gen IS NOT NULL`,

		`CREATE TABLE IF NOT EXISTS vuln_affected (
			id              BIGSERIAL PRIMARY KEY,
			advisory_id     VARCHAR(255) NOT NULL,
			ecosystem       VARCHAR(64) NOT NULL,
			release         VARCHAR(64) NOT NULL DEFAULT '',
			package_name    VARCHAR(512) NOT NULL,
			range_type      VARCHAR(16) NOT NULL,
			introduced      TEXT NOT NULL DEFAULT '',
			fixed           TEXT NOT NULL DEFAULT '',
			last_affected   TEXT NOT NULL DEFAULT '',
			vendor_severity VARCHAR(16) NOT NULL DEFAULT '',
			valid_from_gen  BIGINT NOT NULL,
			valid_to_gen    BIGINT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_affected_package ON vuln_affected (ecosystem, package_name, release)`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_affected_advisory ON vuln_affected (advisory_id)`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_affected_from ON vuln_affected (valid_from_gen)`,
		`CREATE INDEX IF NOT EXISTS idx_vuln_affected_to ON vuln_affected (valid_to_gen) WHERE valid_to_gen IS NOT NULL`,

		`CREATE TABLE IF NOT EXISTS vulnerabilities (
			vuln_id            VARCHAR(255) PRIMARY KEY,
			title              TEXT NOT NULL DEFAULT '',
			description        TEXT NOT NULL DEFAULT '',
			published_at       TIMESTAMPTZ,
			nvd_cvss_v3_vector TEXT NOT NULL DEFAULT '',
			nvd_cvss_v3_score  NUMERIC(3,1),
			nvd_cvss_v4_vector TEXT NOT NULL DEFAULT '',
			nvd_cvss_v4_score  NUMERIC(3,1),
			cwe_ids            TEXT[] NOT NULL DEFAULT '{}',
			kev_added_at       DATE,
			kev_due_date       DATE,
			kev_ransomware     BOOLEAN,
			epss_score         REAL,
			epss_percentile    REAL,
			epss_date          DATE,
			nvd_updated_at     TIMESTAMPTZ,
			kev_updated_at     TIMESTAMPTZ,
			epss_updated_at    TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vulnerabilities_kev ON vulnerabilities (kev_added_at) WHERE kev_added_at IS NOT NULL`,

		`CREATE TABLE IF NOT EXISTS vuln_feed_state (
			feed            VARCHAR(64) NOT NULL,
			scope           VARCHAR(128) NOT NULL DEFAULT '',
			etag            TEXT NOT NULL DEFAULT '',
			cursor          TEXT NOT NULL DEFAULT '',
			last_success_at TIMESTAMPTZ,
			last_error      TEXT NOT NULL DEFAULT '',
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (feed, scope)
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("[Migration 157] %w", err)
		}
	}

	log.Println("[Migration 157] Versioned vulnerability catalog tables created")
	return nil
}
