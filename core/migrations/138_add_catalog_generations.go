package migrations

import (
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration138_AddCatalogGenerations stores authoritative catalog load generations.
func Migration138_AddCatalogGenerations(db *gorm.DB) error {
	log.Println("[Migration 138] Creating catalog generation metadata table...")
	if err := db.AutoMigrate(&models.CatalogGeneration{}); err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_catalog_generations_type_status ON catalog_generations(catalog_type, status)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_catalog_generations_type_activated ON catalog_generations(catalog_type, activated_at DESC)").Error; err != nil {
		return err
	}
	log.Println("[Migration 138] catalog_generations table created")
	return nil
}
