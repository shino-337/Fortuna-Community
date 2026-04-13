package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration119_AddRiskScoreV3Columns adds V3 dimension columns to the
// risk_scores table, enabling the Unified Scorer to store per-dimension
// breakdowns alongside the overall score.
//
// Date: 2026-04-13
//
// Description:
//
//	Phase 2.3 of the Unified Risk Pipeline: adds four new FLOAT columns so
//	the V3 scorer can persist its dimension breakdown without breaking the
//	existing V2 schema.
//
// Columns Added:
//   - capability_exposure_score FLOAT DEFAULT 0
//   - attack_path_score         FLOAT DEFAULT 0
//   - runtime_threat_score      FLOAT DEFAULT 0
//   - blast_radius_score        FLOAT DEFAULT 0
//
// Rollback Plan:
//
//	ALTER TABLE risk_scores DROP COLUMN IF EXISTS capability_exposure_score;
//	ALTER TABLE risk_scores DROP COLUMN IF EXISTS attack_path_score;
//	ALTER TABLE risk_scores DROP COLUMN IF EXISTS runtime_threat_score;
//	ALTER TABLE risk_scores DROP COLUMN IF EXISTS blast_radius_score;
func Migration119_AddRiskScoreV3Columns(db *gorm.DB) error {
	log.Println("[Migration 119] Starting: add V3 dimension columns to risk_scores")

	type column struct {
		name string
		def  string
	}
	columns := []column{
		{"capability_exposure_score", "FLOAT DEFAULT 0"},
		{"attack_path_score", "FLOAT DEFAULT 0"},
		{"runtime_threat_score", "FLOAT DEFAULT 0"},
		{"blast_radius_score", "FLOAT DEFAULT 0"},
	}

	for _, col := range columns {
		if db.Migrator().HasColumn("risk_scores", col.name) {
			log.Printf("[Migration 119] Column %s already exists, skipping", col.name)
			continue
		}
		sql := "ALTER TABLE risk_scores ADD COLUMN IF NOT EXISTS " + col.name + " " + col.def
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
		log.Printf("[Migration 119] ✅ Added column %s", col.name)
	}

	log.Println("[Migration 119] ✅ Completed - V3 dimension columns added to risk_scores")
	return nil
}
