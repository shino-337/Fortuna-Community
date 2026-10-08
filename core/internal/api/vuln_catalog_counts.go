package api

import (
	"time"

	"gorm.io/gorm"
)

// vulnCatalogCounts sizes the vulnerability catalog the matcher reads.
type vulnCatalogCounts struct {
	CVEs       int64 // distinct CVEs
	Ranges     int64 // affected package ranges
	Advisories int64 // advisories (OSV documents)
}

// countVulnCatalog counts the current versions of the versioned catalog. A database the
// versioned loader never filled is counted from the legacy tables (cves,
// package_vulnerabilities, osv_packages) instead.
func countVulnCatalog(db *gorm.DB) (vulnCatalogCounts, error) {
	var out vulnCatalogCounts
	m := db.Migrator()
	if m.HasTable("vuln_advisories") {
		if err := db.Raw(`SELECT COUNT(*) FROM vuln_advisories WHERE valid_to_gen IS NULL`).Scan(&out.Advisories).Error; err != nil {
			return out, err
		}
	}
	if out.Advisories > 0 {
		if err := db.Raw(`SELECT COUNT(DISTINCT ref_id) FROM vuln_advisory_refs WHERE valid_to_gen IS NULL AND ref_kind = 'cve'`).Scan(&out.CVEs).Error; err != nil {
			return out, err
		}
		err := db.Raw(`SELECT COUNT(*) FROM vuln_affected WHERE valid_to_gen IS NULL`).Scan(&out.Ranges).Error
		return out, err
	}

	if m.HasTable("cves") {
		if err := db.Table("cves").Count(&out.CVEs).Error; err != nil {
			return out, err
		}
	}
	if m.HasTable("package_vulnerabilities") {
		if err := db.Table("package_vulnerabilities").Count(&out.Ranges).Error; err != nil {
			return out, err
		}
	}
	if m.HasTable("osv_packages") {
		if err := db.Table("osv_packages").Count(&out.Advisories).Error; err != nil {
			return out, err
		}
	}
	return out, nil
}

// vulnCatalogUpdatedAt returns when the catalog the matcher reads last changed: the activation
// of the active CVE generation, or the newest legacy cves row when the versioned catalog is empty.
func vulnCatalogUpdatedAt(db *gorm.DB, versioned bool) (*time.Time, error) {
	m := db.Migrator()
	if versioned {
		if !m.HasTable("catalog_generations") {
			return nil, nil
		}
		return latestQueryTime(db.Table("catalog_generations").
			Where("catalog_type = ? AND status = ? AND deleted_at IS NULL", "cve", "active"), "activated_at")
	}
	if !m.HasTable("cves") {
		return nil, nil
	}
	return latestQueryTime(db.Table("cves"), "updated_at")
}

// versionedCatalogLoaded reports whether the versioned catalog has current advisories.
func versionedCatalogLoaded(db *gorm.DB) (bool, error) {
	if !db.Migrator().HasTable("vuln_advisories") {
		return false, nil
	}
	var found bool
	err := db.Raw(`SELECT EXISTS (SELECT 1 FROM vuln_advisories WHERE valid_to_gen IS NULL)`).Scan(&found).Error
	return found, err
}
