package migrations

import (
	"encoding/json"
	"log"

	"gorm.io/gorm"
)

// Migration051_SeedPromotionRules seeds initial promotion rules
func Migration051_SeedPromotionRules(db *gorm.DB) error {
	log.Println("Running migration 051: Seed promotion_rules")
	if !shouldRunSeedMigrations() {
		log.Println("[Migration 051] seed disabled (FORTUNA_ENABLE_SEED_DATA not set); set FORTUNA_ENABLE_SEED_DATA=true for PCE promotion rules")
		return nil
	}

	// Check if table exists
	if !db.Migrator().HasTable("promotion_rules") {
		log.Println("[Migration 051] promotion_rules table does not exist, skipping seed")
		return nil
	}

	rules := []map[string]interface{}{
		// PROC_ROOT_PIVOT → ESC_HOSTPATH_NODE (confirmed)
		{
			"capability_id":         "ESC_HOSTPATH_NODE",
			"signal_type":           "PROC_ROOT_PIVOT",
			"min_occurrences":       1,
			"required_capabilities": []string{},
			"promote_to":            "confirmed",
			"confidence_boost":      0.2,
		},
		// PROC_ROOT_PIVOT + SYS_ADMIN → ESC_HOSTPATH_NODE (exploited)
		{
			"capability_id":         "ESC_HOSTPATH_NODE",
			"signal_type":           "PROC_ROOT_PIVOT",
			"min_occurrences":       2,
			"required_capabilities": []string{"SYS_ADMIN"},
			"promote_to":            "exploited",
			"confidence_boost":      0.3,
		},
		// FS_ESCAPE_ATTEMPT → ESC_HOSTPATH_NODE (exploited)
		{
			"capability_id":         "ESC_HOSTPATH_NODE",
			"signal_type":           "FS_ESCAPE_ATTEMPT",
			"min_occurrences":       1,
			"required_capabilities": []string{},
			"promote_to":            "exploited",
			"confidence_boost":      0.3,
		},
		// NAMESPACE_ESCAPE → ESC_HOSTPID_POD or ESC_HOSTIPC_POD (confirmed)
		{
			"capability_id":         "ESC_HOSTPID_POD",
			"signal_type":           "NAMESPACE_ESCAPE",
			"min_occurrences":       1,
			"required_capabilities": []string{},
			"promote_to":            "confirmed",
			"confidence_boost":      0.2,
		},
		{
			"capability_id":         "ESC_HOSTIPC_POD",
			"signal_type":           "NAMESPACE_ESCAPE",
			"min_occurrences":       1,
			"required_capabilities": []string{},
			"promote_to":            "confirmed",
			"confidence_boost":      0.2,
		},
		// CAPABILITY_MISUSE → ESC_PRIV_POD (confirmed)
		{
			"capability_id":         "ESC_PRIV_POD",
			"signal_type":           "CAPABILITY_MISUSE",
			"min_occurrences":       1,
			"required_capabilities": []string{"SYS_ADMIN"},
			"promote_to":            "confirmed",
			"confidence_boost":      0.2,
		},
		// PROC_ROOT_PIVOT → ESC_RUNTIME_PROBE (confirmed)
		{
			"capability_id":         "ESC_RUNTIME_PROBE",
			"signal_type":           "PROC_ROOT_PIVOT",
			"min_occurrences":       1,
			"required_capabilities": []string{},
			"promote_to":            "confirmed",
			"confidence_boost":      0.2,
		},
		// Multiple PROC_ROOT_PIVOT → ESC_RUNTIME_ACTIVE (exploited)
		{
			"capability_id":         "ESC_RUNTIME_ACTIVE",
			"signal_type":           "PROC_ROOT_PIVOT",
			"min_occurrences":       3,
			"required_capabilities": []string{},
			"promote_to":            "exploited",
			"confidence_boost":      0.3,
		},
		// FS_ESCAPE_ATTEMPT → ESC_RUNTIME_ACTIVE (exploited)
		{
			"capability_id":         "ESC_RUNTIME_ACTIVE",
			"signal_type":           "FS_ESCAPE_ATTEMPT",
			"min_occurrences":       1,
			"required_capabilities": []string{},
			"promote_to":            "exploited",
			"confidence_boost":      0.3,
		},
	}

	for _, r := range rules {
		requiredCaps := r["required_capabilities"].([]string)
		requiredCapsJSON, err := json.Marshal(requiredCaps)
		if err != nil {
			log.Printf("[Migration 051] Failed to marshal required_capabilities for %s + %s: %v",
				r["capability_id"], r["signal_type"], err)
			continue
		}

		sql := `
			INSERT INTO promotion_rules (
				capability_id, signal_type, min_occurrences, 
				required_capabilities, promote_to, confidence_boost
			) VALUES ($1, $2, $3, $4::jsonb, $5, $6)
			ON CONFLICT (capability_id, signal_type, promote_to) DO UPDATE SET
				min_occurrences = EXCLUDED.min_occurrences,
				required_capabilities = EXCLUDED.required_capabilities,
				confidence_boost = EXCLUDED.confidence_boost,
				updated_at = CURRENT_TIMESTAMP
		`

		if err := db.Exec(sql,
			r["capability_id"],
			r["signal_type"],
			r["min_occurrences"],
			string(requiredCapsJSON),
			r["promote_to"],
			r["confidence_boost"],
		).Error; err != nil {
			log.Printf("[Migration 051] Failed to seed promotion rule %s + %s: %v",
				r["capability_id"], r["signal_type"], err)
			continue
		}
	}

	log.Println("[Migration 051] ✅ Completed successfully")
	return nil
}
