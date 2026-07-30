package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration075_AddRiskRulesTable creates risk_rules table for CRUD risk rules (engine can load from DB).
func Migration075_AddRiskRulesTable(db *gorm.DB) error {
	log.Println("Running migration 075: Add risk_rules table")

	if err := db.AutoMigrate(&models.RiskRule{}); err != nil {
		return err
	}
	log.Println("[Migration 075] Completed successfully")
	return nil
}
