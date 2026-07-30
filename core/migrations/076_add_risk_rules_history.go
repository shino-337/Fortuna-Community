package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration076_AddRiskRulesHistoryTable creates risk_rules_history for rule versioning (Phase 3).
// Uses execDDL instead of AutoMigrate: GORM can return "insufficient arguments" on introspection
// (SELECT * FROM risk_rules_history LIMIT 1) with some PostgreSQL driver builds.
func Migration076_AddRiskRulesHistoryTable(db *gorm.DB) error {
	log.Println("Running migration 076: Add risk_rules_history table")

	if db.Dialector.Name() != "postgres" {
		return db.AutoMigrate(&models.RiskRuleHistory{})
	}

	if err := execDDL(db, `
CREATE TABLE IF NOT EXISTS risk_rules_history (
	id BIGSERIAL PRIMARY KEY,
	rule_id VARCHAR(128) NOT NULL,
	version INTEGER NOT NULL,
	snapshot JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_risk_rules_history_rule_id ON risk_rules_history(rule_id);
`); err != nil {
		return err
	}
	log.Println("[Migration 076] Completed successfully")
	return nil
}
