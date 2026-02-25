package migrations

import (
	"os"
	"strings"
)

// shouldRunSeedMigrations returns true when FORTUNA_ENABLE_SEED_DATA is set (true/1/yes/on).
// Used by migrations that insert reference data (capability_metadata, promotion_rules) required for
// Capability Catalog and PCE. Set FORTUNA_ENABLE_SEED_DATA=true in Core deployment to populate catalog.
func shouldRunSeedMigrations() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("FORTUNA_ENABLE_SEED_DATA")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
