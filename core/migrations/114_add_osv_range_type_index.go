package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration114_AddOSVRangeTypeIndex adds an index on osv_ranges.range_type to speed
// filtering when joins return many range rows (SEMVER vs ECOSYSTEM).
func Migration114_AddOSVRangeTypeIndex(db *gorm.DB) error {
	log.Println("Running migration 114: index on osv_ranges.range_type")

	if !db.Migrator().HasTable("osv_ranges") {
		log.Println("Migration 114: osv_ranges missing, skipping")
		return nil
	}

	if err := db.Exec(`
CREATE INDEX IF NOT EXISTS idx_osv_ranges_range_type ON osv_ranges (range_type);
`).Error; err != nil {
		log.Printf("Migration 114: failed to create idx_osv_ranges_range_type: %v", err)
		return err
	}

	log.Println("Migration 114 completed: idx_osv_ranges_range_type")
	return nil
}
