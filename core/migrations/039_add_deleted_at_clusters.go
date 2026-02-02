package migrations

import "gorm.io/gorm"

// Migration039_AddDeletedAtToClusters ensures clusters has deleted_at for soft deletes.
func Migration039_AddDeletedAtToClusters(db *gorm.DB) error {
	if err := db.Exec(`ALTER TABLE clusters ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_clusters_deleted_at ON clusters(deleted_at)`).Error; err != nil {
		return err
	}
	return nil
}
