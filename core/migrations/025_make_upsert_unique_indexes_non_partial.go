package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration025_MakeUpsertUniqueIndexesNonPartial ensures ON CONFLICT inference works by creating
// non-partial unique indexes for upsert keys. Postgres cannot infer a *partial* unique index
// unless the ON CONFLICT clause includes the same predicate, which GORM does not emit here.
func Migration025_MakeUpsertUniqueIndexesNonPartial(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 025] Ensure non-partial unique indexes for SBOM/CVE upserts")
	log.Println("========================================")

	// SBOM components: (sbom_id, purl)
	if err := db.Exec(`
DELETE FROM sbom_components a
USING sbom_components b
WHERE a.id > b.id
  AND a.sbom_id = b.sbom_id
  AND a.purl = b.purl;
`).Error; err != nil {
		return fmt.Errorf("dedup sbom_components for non-partial unique index: %w", err)
	}
	if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_sbom_components_unique_sbom_purl_all
  ON sbom_components(sbom_id, purl);
`).Error; err != nil {
		return fmt.Errorf("create unique index sbom_components(sbom_id,purl): %w", err)
	}

	// CVE matches: (sbom_id, component_id, cve_id)
	if err := db.Exec(`
DELETE FROM cve_matches a
USING cve_matches b
WHERE a.id > b.id
  AND a.sbom_id = b.sbom_id
  AND a.component_id = b.component_id
  AND a.cve_id = b.cve_id;
`).Error; err != nil {
		return fmt.Errorf("dedup cve_matches for non-partial unique index: %w", err)
	}
	if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_cve_matches_unique_sbom_component_cve_all
  ON cve_matches(sbom_id, component_id, cve_id);
`).Error; err != nil {
		return fmt.Errorf("create unique index cve_matches(sbom_id,component_id,cve_id): %w", err)
	}

	// Pod image scans: (pod_uid, container_name)
	if err := db.Exec(`
DELETE FROM pod_image_scans a
USING pod_image_scans b
WHERE a.id > b.id
  AND a.pod_uid = b.pod_uid
  AND a.container_name = b.container_name;
`).Error; err != nil {
		return fmt.Errorf("dedup pod_image_scans for non-partial unique index: %w", err)
	}
	if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_image_scans_unique_pod_uid_container_name_all
  ON pod_image_scans(pod_uid, container_name);
`).Error; err != nil {
		return fmt.Errorf("create unique index pod_image_scans(pod_uid,container_name): %w", err)
	}

	log.Println("[Migration 025] ✅ Completed")
	return nil
}


