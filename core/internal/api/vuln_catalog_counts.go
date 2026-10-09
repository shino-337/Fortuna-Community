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

// countVulnCatalog counts the current versions of the versioned catalog.
func countVulnCatalog(db *gorm.DB) (vulnCatalogCounts, error) {
	var out vulnCatalogCounts
	if !db.Migrator().HasTable("vuln_advisories") {
		return out, nil
	}
	if err := db.Raw(`SELECT COUNT(*) FROM vuln_advisories WHERE valid_to_gen IS NULL`).Scan(&out.Advisories).Error; err != nil {
		return out, err
	}
	if out.Advisories == 0 {
		return out, nil
	}
	if err := db.Raw(`SELECT COUNT(DISTINCT ref_id) FROM vuln_advisory_refs WHERE valid_to_gen IS NULL AND ref_kind = 'cve'`).Scan(&out.CVEs).Error; err != nil {
		return out, err
	}
	err := db.Raw(`SELECT COUNT(*) FROM vuln_affected WHERE valid_to_gen IS NULL`).Scan(&out.Ranges).Error
	return out, err
}

// vulnCatalogUpdatedAt returns when the catalog the matcher reads last changed: the activation
// of the active CVE generation.
func vulnCatalogUpdatedAt(db *gorm.DB) (*time.Time, error) {
	if !db.Migrator().HasTable("catalog_generations") {
		return nil, nil
	}
	return latestQueryTime(db.Table("catalog_generations").
		Where("catalog_type = ? AND status = ? AND deleted_at IS NULL", "cve", "active"), "activated_at")
}
