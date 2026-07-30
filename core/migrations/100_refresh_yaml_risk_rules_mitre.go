package migrations

import "log"

import "gorm.io/gorm"

// Migration100_RefreshYAMLRiskRulesMITRE is now a no-op.
// YAML rule sync is handled by Migration097 (runs once per deploy).
func Migration100_RefreshYAMLRiskRulesMITRE(db *gorm.DB) error {
	log.Println("[migration 100] no-op (consolidated into 097)")
	return nil
}
