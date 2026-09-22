package migrations

import (
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

func EnsureInventoryCollection(db *gorm.DB) error {
	return db.AutoMigrate(&models.InventoryCollection{})
}
