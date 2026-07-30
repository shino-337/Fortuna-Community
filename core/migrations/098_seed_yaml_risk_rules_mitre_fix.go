package migrations

import "log"

import "gorm.io/gorm"

// Migration098_SeedYAMLRiskRulesMITREFix is now a no-op.
// The work (cleanup empty rule_id + upsert all YAML rules) is handled by Migration097.
// Keeping the function so the migration slice index stays stable.
func Migration098_SeedYAMLRiskRulesMITREFix(db *gorm.DB) error {
	log.Println("[migration 098] no-op (consolidated into 097)")
	return nil
}
