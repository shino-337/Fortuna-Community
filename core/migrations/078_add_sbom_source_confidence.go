package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration078_AddSBOMSourceConfidence adds sbom_source and confidence to sboms (Finding #8.4).
// Core uses these to know if SBOM is from parsers vs distroless-heuristic and to tune CVE matching confidence.
func Migration078_AddSBOMSourceConfidence(db *gorm.DB) error {
	log.Println("[Migration 078] Add sbom_source and confidence to sboms (Finding #8.4)")

	if !db.Migrator().HasTable("sboms") {
		log.Println("[Migration 078] sboms table does not exist, skipping")
		return nil
	}

	var hasSource, hasConf, hasStatus int64
	_ = db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'sboms' AND column_name = 'sbom_source'").Scan(&hasSource).Error
	_ = db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'sboms' AND column_name = 'confidence'").Scan(&hasConf).Error
	_ = db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'sboms' AND column_name = 'status'").Scan(&hasStatus).Error

	if hasSource == 0 {
		if err := db.Exec("ALTER TABLE sboms ADD COLUMN sbom_source VARCHAR(64) DEFAULT ''").Error; err != nil {
			return err
		}
		log.Println("[Migration 078] ✅ Added sbom_source column")
	}
	if hasConf == 0 {
		if err := db.Exec("ALTER TABLE sboms ADD COLUMN confidence VARCHAR(32) DEFAULT ''").Error; err != nil {
			return err
		}
		log.Println("[Migration 078] ✅ Added confidence column")
	}

	if hasStatus == 0 {
		log.Println("[Migration 078] Adding status column to sboms (pending|finalized)")
		if err := db.Exec("ALTER TABLE sboms ADD COLUMN status VARCHAR(20) DEFAULT 'pending'").Error; err != nil {
			return err
		}
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_sboms_status ON sboms(status)").Error; err != nil {
			log.Printf("[Migration 078] ⚠️ Failed to create idx_sboms_status: %v", err)
		} else {
			log.Println("[Migration 078] ✅ Added status column and index")
		}
	} else {
		log.Println("[Migration 078] status column already exists on sboms, skipping")
	}

	return nil
}
