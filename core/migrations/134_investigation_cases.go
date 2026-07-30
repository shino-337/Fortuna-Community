package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration134_InvestigationCases adds persisted investigation workspace cases.
func Migration134_InvestigationCases(db *gorm.DB) error {
	log.Println("Running migration 134: investigation_cases")
	return db.AutoMigrate(&models.InvestigationCase{})
}
