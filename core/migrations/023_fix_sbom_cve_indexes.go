package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration023_FixSBOMCVEIndexes adds unique indexes needed for safe ON CONFLICT upserts
// and removes duplicates that would prevent unique index creation.
func Migration023_FixSBOMCVEIndexes(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 023] Fix SBOM/CVE unique indexes (dedup + constraints)")
	log.Println("========================================")

	// Guard: only run if tables exist
	var sbomComponentsExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'sbom_components')").Scan(&sbomComponentsExists).Error; err != nil {
		return fmt.Errorf("check sbom_components table exists: %w", err)
	}
	var cveMatchesExists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'cve_matches')").Scan(&cveMatchesExists).Error; err != nil {
		return fmt.Errorf("check cve_matches table exists: %w", err)
	}

	if !sbomComponentsExists && !cveMatchesExists {
		log.Println("[Migration 023] sbom_components and cve_matches do not exist, skipping")
		return nil
	}

	if sbomComponentsExists {
		// 1) Deduplicate sbom_components by (sbom_id, purl) for active (non-deleted) rows
		dedupSBOMComponents := `
DELETE FROM sbom_components a
USING sbom_components b
WHERE a.id > b.id
  AND a.sbom_id = b.sbom_id
  AND a.purl = b.purl
  AND a.deleted_at IS NULL
  AND b.deleted_at IS NULL
  AND a.purl IS NOT NULL;
`
		if err := db.Exec(dedupSBOMComponents).Error; err != nil {
			return fmt.Errorf("dedup sbom_components failed: %w", err)
		}

		// 2) Create unique index for sbom_components upsert path
		createUniqueSBOMComponents := `
CREATE UNIQUE INDEX IF NOT EXISTS idx_sbom_components_unique_sbom_purl
  ON sbom_components(sbom_id, purl)
  WHERE deleted_at IS NULL;
`
		if err := db.Exec(createUniqueSBOMComponents).Error; err != nil {
			return fmt.Errorf("create unique index sbom_components(sbom_id,purl) failed: %w", err)
		}
	}

	if cveMatchesExists {
		// 3) Deduplicate cve_matches by (sbom_id, package_name, cve_id) for active rows
		dedupCVEMatches := `
DELETE FROM cve_matches a
USING cve_matches b
WHERE a.id > b.id
  AND a.sbom_id = b.sbom_id
  AND a.package_name = b.package_name
  AND a.cve_id = b.cve_id
  AND a.deleted_at IS NULL
  AND b.deleted_at IS NULL;
`
		if err := db.Exec(dedupCVEMatches).Error; err != nil {
			return fmt.Errorf("dedup cve_matches failed: %w", err)
		}

		// 4) Create unique index for cve_matches upsert path (using package_name instead of component_id)
		createUniqueCVEMatches := `
CREATE UNIQUE INDEX IF NOT EXISTS idx_cve_matches_unique_sbom_package_cve
  ON cve_matches(sbom_id, package_name, cve_id)
  WHERE deleted_at IS NULL;
`
		if err := db.Exec(createUniqueCVEMatches).Error; err != nil {
			return fmt.Errorf("create unique index cve_matches(sbom_id,package_name,cve_id) failed: %w", err)
		}
	}

	log.Println("[Migration 023] ✅ Completed")
	return nil
}


