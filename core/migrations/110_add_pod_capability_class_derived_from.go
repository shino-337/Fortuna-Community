package migrations

import "gorm.io/gorm"

// Migration110_AddPodCapabilityClassDerivedFrom extends pod_capabilities for compatibility strategy.
func Migration110_AddPodCapabilityClassDerivedFrom(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	_ = db.Exec("ALTER TABLE pod_capabilities ADD COLUMN IF NOT EXISTS capability_class VARCHAR(30) DEFAULT 'effective'").Error
	_ = db.Exec("ALTER TABLE pod_capabilities ADD COLUMN IF NOT EXISTS derived_from JSONB").Error
	_ = db.Exec("UPDATE pod_capabilities SET capability_class = COALESCE(NULLIF(capability_class,''), 'effective')").Error
	_ = db.Exec("UPDATE pod_capabilities SET derived_from = '{}'::jsonb WHERE derived_from IS NULL").Error
	_ = db.Exec("CREATE INDEX IF NOT EXISTS idx_pod_capabilities_class ON pod_capabilities(capability_class)").Error
	return nil
}
