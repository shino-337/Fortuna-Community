package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration048_AddPromotionRulesTable creates promotion_rules table for signal → state transitions
func Migration048_AddPromotionRulesTable(db *gorm.DB) error {
	log.Println("Running migration 048: Add promotion_rules table")

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS promotion_rules (
			id SERIAL PRIMARY KEY,
			capability_id VARCHAR(100) NOT NULL,
			signal_type VARCHAR(100) NOT NULL,
			min_occurrences INTEGER DEFAULT 1,
			required_capabilities JSONB,
			promote_to VARCHAR(20) NOT NULL CHECK (promote_to IN ('detected', 'confirmed', 'exploited', 'chained')),
			confidence_boost FLOAT DEFAULT 0.1 CHECK (confidence_boost >= 0 AND confidence_boost <= 1),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(capability_id, signal_type, promote_to)
		);
		CREATE INDEX IF NOT EXISTS idx_promotion_rules_capability_id ON promotion_rules(capability_id);
		CREATE INDEX IF NOT EXISTS idx_promotion_rules_signal_type ON promotion_rules(signal_type);
		CREATE INDEX IF NOT EXISTS idx_promotion_rules_promote_to ON promotion_rules(promote_to);
	`).Error; err != nil {
		return err
	}

	log.Println("[Migration 048] ✅ Completed successfully")
	return nil
}
