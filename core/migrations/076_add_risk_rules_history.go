package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration076_AddRiskRulesHistoryTable creates risk_rules_history for rule versioning (Phase 3).
func Migration076_AddRiskRulesHistoryTable(db *gorm.DB) error {
	log.Println("Running migration 076: Add risk_rules_history table")
	if err := db.AutoMigrate(&models.RiskRuleHistory{}); err != nil {
		return err
	}
	log.Println("[Migration 076] Completed successfully")
	return nil
}
