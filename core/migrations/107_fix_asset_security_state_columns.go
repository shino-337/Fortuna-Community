package migrations

import (
	"log"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Migration107_FixAssetSecurityStateColumnNames renames columns that may have been created
// with acronym-splitting (host_p_id/host_ip_c/signal_total24h) to the intended names.
func Migration107_FixAssetSecurityStateColumnNames(db *gorm.DB) error {
	log.Println("Running migration 107: Fix asset_security_state column names")

	if !db.Migrator().HasTable(&models.AssetSecurityState{}) {
		return nil
	}

	// host_pid
	if db.Migrator().HasColumn(&models.AssetSecurityState{}, "host_p_id") && !db.Migrator().HasColumn(&models.AssetSecurityState{}, "host_pid") {
		if err := db.Exec(`ALTER TABLE asset_security_state RENAME COLUMN host_p_id TO host_pid`).Error; err != nil {
			return err
		}
	}
	// host_ipc
	if db.Migrator().HasColumn(&models.AssetSecurityState{}, "host_ip_c") && !db.Migrator().HasColumn(&models.AssetSecurityState{}, "host_ipc") {
		if err := db.Exec(`ALTER TABLE asset_security_state RENAME COLUMN host_ip_c TO host_ipc`).Error; err != nil {
			return err
		}
	}
	// signal_total_24h
	if db.Migrator().HasColumn(&models.AssetSecurityState{}, "signal_total24h") && !db.Migrator().HasColumn(&models.AssetSecurityState{}, "signal_total_24h") {
		if err := db.Exec(`ALTER TABLE asset_security_state RENAME COLUMN signal_total24h TO signal_total_24h`).Error; err != nil {
			return err
		}
	}
	return nil
}
