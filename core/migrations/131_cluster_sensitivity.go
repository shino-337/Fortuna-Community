package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration131_ClusterSensitivity adds clusters.sensitivity for resource classification (governance foundation).
func Migration131_ClusterSensitivity(db *gorm.DB) error {
	log.Println("Running migration 131: clusters.sensitivity")
	if !db.Migrator().HasTable(&models.Cluster{}) {
		log.Println("Migration 131: clusters table missing, skipping")
		return nil
	}
	if db.Migrator().HasColumn(&models.Cluster{}, "Sensitivity") {
		log.Println("Migration 131: sensitivity already present")
		return nil
	}
	switch db.Dialector.Name() {
	case "sqlite":
		if err := db.Exec("ALTER TABLE clusters ADD COLUMN sensitivity TEXT DEFAULT 'internal'").Error; err != nil {
			return err
		}
	default:
		if err := db.Exec("ALTER TABLE clusters ADD COLUMN sensitivity VARCHAR(32) DEFAULT 'internal'").Error; err != nil {
			return err
		}
	}
	log.Println("Migration 131 completed successfully")
	return nil
}
