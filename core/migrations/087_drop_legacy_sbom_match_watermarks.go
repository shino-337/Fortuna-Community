package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration087_DropLegacySBOMMatchWatermarks removes the unused sbom_match_watermarks table
// introduced briefly before replay guard was consolidated into sbom_processing_state (086).
func Migration087_DropLegacySBOMMatchWatermarks(db *gorm.DB) error {
	log.Println("[Migration 087] Dropping legacy sbom_match_watermarks if present")
	if err := db.Exec(`DROP TABLE IF EXISTS sbom_match_watermarks;`).Error; err != nil {
		return fmt.Errorf("[Migration 087] failed to drop sbom_match_watermarks: %w", err)
	}
	log.Println("[Migration 087] Completed dropping legacy sbom_match_watermarks")
	return nil
}
