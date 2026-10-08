package api

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The catalog health counts the versioned catalog once it is loaded, and the legacy tables
// only before that.
func TestCountVulnCatalogPrefersVersionedCatalog(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	for _, stmt := range []string{
		`CREATE TABLE cves (id INTEGER PRIMARY KEY, updated_at DATETIME)`,
		`CREATE TABLE package_vulnerabilities (id INTEGER PRIMARY KEY)`,
		`INSERT INTO cves (id, updated_at) VALUES (1, '2024-01-01 00:00:00'), (2, '2024-01-02 00:00:00')`,
		`INSERT INTO package_vulnerabilities (id) VALUES (1)`,
		`CREATE TABLE vuln_advisories (advisory_id TEXT, valid_to_gen INTEGER)`,
		`CREATE TABLE vuln_advisory_refs (advisory_id TEXT, ref_id TEXT, ref_kind TEXT, valid_to_gen INTEGER)`,
		`CREATE TABLE vuln_affected (advisory_id TEXT, valid_to_gen INTEGER)`,
		`CREATE TABLE catalog_generations (id INTEGER PRIMARY KEY, catalog_type TEXT, status TEXT, activated_at DATETIME, deleted_at DATETIME)`,
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}

	got, err := countVulnCatalog(db)
	require.NoError(t, err)
	require.Equal(t, vulnCatalogCounts{CVEs: 2, Ranges: 1}, got)
	versioned, err := versionedCatalogLoaded(db)
	require.NoError(t, err)
	require.False(t, versioned)

	activated := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, stmt := range []string{
		`INSERT INTO vuln_advisories VALUES ('DSA-1', NULL), ('GHSA-1', NULL), ('OLD-1', 3)`,
		`INSERT INTO vuln_advisory_refs VALUES ('DSA-1', 'CVE-1', 'cve', NULL), ('DSA-1', 'CVE-2', 'cve', NULL),
		   ('GHSA-1', 'CVE-1', 'cve', NULL), ('GHSA-1', 'GHSA-1', 'ghsa', NULL), ('OLD-1', 'CVE-9', 'cve', 3)`,
		`INSERT INTO vuln_affected VALUES ('DSA-1', NULL), ('DSA-1', NULL), ('GHSA-1', NULL), ('OLD-1', 3)`,
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO catalog_generations (catalog_type, status, activated_at) VALUES ('cve', 'active', ?)`, activated).Error)

	got, err = countVulnCatalog(db)
	require.NoError(t, err)
	require.Equal(t, vulnCatalogCounts{CVEs: 2, Ranges: 3, Advisories: 2}, got)
	versioned, err = versionedCatalogLoaded(db)
	require.NoError(t, err)
	require.True(t, versioned)
	at, err := vulnCatalogUpdatedAt(db, versioned)
	require.NoError(t, err)
	require.NotNil(t, at)
	require.True(t, at.Equal(activated))
}
