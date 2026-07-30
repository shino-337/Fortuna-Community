package migrations

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// Migration084_AddSBOMComponentTrustFields adds fields used for trust-boundary enforcement:
// - original_purl (optional)
// - purl_validated (bool)
// - trust_level (high|medium|low)
func Migration084_AddSBOMComponentTrustFields(db *gorm.DB) error {
	log.Println("[Migration 084] Adding SBOM component trust fields")

	if !db.Migrator().HasTable("sbom_components") {
		log.Println("[Migration 084] sbom_components table does not exist, skipping")
		return nil
	}

	if db.Dialector.Name() != "postgres" {
		log.Println("[Migration 084] skipping: PostgreSQL-specific DDL")
		return nil
	}

	// Use execDDL + unique dollar-quote tag to avoid GORM "insufficient arguments" with $$.
	if err := execDDL(db, `
DO $m084$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'sbom_components'
      AND column_name = 'original_purl'
  ) THEN
    ALTER TABLE sbom_components ADD COLUMN original_purl VARCHAR(512);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'sbom_components'
      AND column_name = 'purl_validated'
  ) THEN
    ALTER TABLE sbom_components ADD COLUMN purl_validated BOOLEAN NOT NULL DEFAULT true;
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name = 'sbom_components'
      AND column_name = 'trust_level'
  ) THEN
    ALTER TABLE sbom_components ADD COLUMN trust_level VARCHAR(10) NOT NULL DEFAULT 'high';
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'sbom_components_trust_level_check'
      AND conrelid = 'sbom_components'::regclass
  ) THEN
    ALTER TABLE sbom_components ADD CONSTRAINT sbom_components_trust_level_check
      CHECK (trust_level IN ('high','medium','low'));
  END IF;
END$m084$;
`); err != nil {
		return fmt.Errorf("[Migration 084] failed: %w", err)
	}

	log.Println("[Migration 084] Completed SBOM component trust fields")
	return nil
}

