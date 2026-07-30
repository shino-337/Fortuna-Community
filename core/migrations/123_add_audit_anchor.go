package migrations

import (
	"log"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Migration123_AddAuditAnchor creates append-only audit anchors for hash-chain checkpoints.
func Migration123_AddAuditAnchor(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.AuditAnchor{}); err != nil {
		return err
	}
	log.Println("Migration123: audit_anchor table ensured")
	return nil
}

