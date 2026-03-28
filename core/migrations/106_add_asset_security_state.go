package migrations

import (
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration106_AddAssetSecurityState adds asset_security_state minimal table.
func Migration106_AddAssetSecurityState(db *gorm.DB) error {
	log.Println("Running migration 106: Add asset_security_state table")
	if err := db.AutoMigrate(&models.AssetSecurityState{}); err != nil {
		return err
	}
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_asset_security_state_pod_uid_updated ON asset_security_state(pod_uid, updated_at DESC)").Error
	return nil
}

