package migrations

import "log"

import "gorm.io/gorm"

// Migration099_RefreshYAMLRiskRulesMITRE is now a no-op.
// YAML rule sync is handled by Migration097 (runs once per deploy).
func Migration099_RefreshYAMLRiskRulesMITRE(db *gorm.DB) error {
	log.Println("[migration 099] no-op (consolidated into 097)")
	return nil
}
