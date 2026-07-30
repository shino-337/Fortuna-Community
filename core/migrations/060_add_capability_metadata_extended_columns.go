package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration060_AddCapabilityMetadataExtendedColumns adds extended columns from Capability Specification (MITRE ATT&CK):
// name, summary, full_description, mitre_tactic, mitre_technique, mitre_subtechnique, kill_chain_stage,
// technical_indicators, impact, recommended_mitigations, false_positive_considerations, references (all JSONB for arrays).
func Migration060_AddCapabilityMetadataExtendedColumns(db *gorm.DB) error {
	log.Println("Running migration 060: Add capability_metadata extended columns (spec MITRE ATT&CK)")

	if !db.Migrator().HasTable("capability_metadata") {
		log.Println("[Migration 060] capability_metadata table does not exist, skipping")
		return nil
	}

	columns := []struct {
		name string
		sql  string
	}{
		{"name", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS name VARCHAR(200) DEFAULT ''"},
		{"summary", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS summary TEXT DEFAULT ''"},
		{"full_description", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS full_description TEXT DEFAULT ''"},
		{"mitre_tactic", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS mitre_tactic VARCHAR(100) DEFAULT ''"},
		{"mitre_technique", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS mitre_technique VARCHAR(50) DEFAULT ''"},
		{"mitre_subtechnique", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS mitre_subtechnique VARCHAR(50) DEFAULT ''"},
		{"kill_chain_stage", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS kill_chain_stage VARCHAR(80) DEFAULT ''"},
		{"technical_indicators", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS technical_indicators JSONB DEFAULT '[]'"},
		{"impact", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS impact JSONB DEFAULT '[]'"},
		{"recommended_mitigations", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS recommended_mitigations JSONB DEFAULT '[]'"},
		{"false_positive_considerations", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS false_positive_considerations JSONB DEFAULT '[]'"},
		{"refs", "ALTER TABLE capability_metadata ADD COLUMN IF NOT EXISTS refs JSONB DEFAULT '[]'"},
	}

	for _, c := range columns {
		if err := db.Exec(c.sql).Error; err != nil {
			log.Printf("[Migration 060] Warning adding column %s: %v", c.name, err)
			// Continue; column may already exist
		}
	}

	log.Println("[Migration 060] ✅ Completed successfully")
	return nil
}
