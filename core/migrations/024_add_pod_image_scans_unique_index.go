package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration024_AddPodImageScansUniqueIndex adds the unique index required by SBOMService.UpsertPodImageScan
// so ON CONFLICT (pod_uid, container_name) works reliably.
func Migration024_AddPodImageScansUniqueIndex(db *gorm.DB) error {
	log.Println("========================================")
	log.Println("[Migration 024] Add pod_image_scans unique index (pod_uid, container_name)")
	log.Println("========================================")

	var exists bool
	if err := db.Raw("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'pod_image_scans')").Scan(&exists).Error; err != nil {
		return fmt.Errorf("check pod_image_scans exists: %w", err)
	}
	if !exists {
		log.Println("[Migration 024] pod_image_scans does not exist, skipping")
		return nil
	}

	// Deduplicate active rows that would block unique index creation.
	dedup := `
DELETE FROM pod_image_scans a
USING pod_image_scans b
WHERE a.id > b.id
  AND a.pod_uid = b.pod_uid
  AND a.container_name = b.container_name
  AND a.deleted_at IS NULL
  AND b.deleted_at IS NULL;
`
	if err := db.Exec(dedup).Error; err != nil {
		return fmt.Errorf("dedup pod_image_scans failed: %w", err)
	}

	createIdx := `
CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_image_scans_unique_pod_uid_container_name
  ON pod_image_scans(pod_uid, container_name)
  WHERE deleted_at IS NULL;
`
	if err := db.Exec(createIdx).Error; err != nil {
		return fmt.Errorf("create unique index pod_image_scans(pod_uid,container_name) failed: %w", err)
	}

	log.Println("[Migration 024] ✅ Completed")
	return nil
}


