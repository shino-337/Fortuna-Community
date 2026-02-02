package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration052_SBOMOneRowPerPod drops the unique constraint on sboms(image_digest)
// so that each pod can have its own SBOM row (one row per pod_uid).
// This fixes the UI showing only one pod per image instead of all pods.
func Migration052_SBOMOneRowPerPod(db *gorm.DB) error {
	log.Println("[Migration 052] SBOM: allow one row per pod (drop unique on image_digest)")

	if !db.Migrator().HasTable("sboms") {
		log.Println("[Migration 052] sboms table does not exist, skipping")
		return nil
	}

	// Drop unique constraint on image_digest so multiple rows can share same image (different pods)
	var hasConstraint bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_constraint 
			WHERE conname = 'sboms_image_digest_key'
		)
	`).Scan(&hasConstraint).Error; err != nil {
		log.Printf("[Migration 052] ⚠️  Error checking constraint: %v", err)
		return err
	}
	if hasConstraint {
		if err := db.Exec(`ALTER TABLE sboms DROP CONSTRAINT IF EXISTS sboms_image_digest_key`).Error; err != nil {
			log.Printf("[Migration 052] ⚠️  Error dropping constraint: %v", err)
			return err
		}
		log.Println("[Migration 052] ✅ Dropped unique constraint sboms_image_digest_key")
	}

	// Ensure non-unique index on image_digest for lookups
	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_sboms_image_digest ON sboms(image_digest);
	`).Error; err != nil {
		log.Printf("[Migration 052] ⚠️  Error creating index: %v", err)
	}
	// Unique index on (pod_uid, created_at) not needed; GetSBOMList uses DISTINCT ON (pod_uid)

	log.Println("[Migration 052] ✅ Completed")
	return nil
}
