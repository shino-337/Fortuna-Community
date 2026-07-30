package migrations

import (
	"log"

	"gorm.io/gorm"
)

// Migration118_AddAttackPathsTable creates the attack_paths persistence table.
//
// Date: 2026-04-13
//
// Description:
//
//	Persists computed attack paths so they can be read by the Unified Scorer
//	(Layer 4) and displayed in the Dashboard without re-computing on every
//	request.  Paths are keyed by (pod_uid, path_id) and upserted on each
//	re-computation so stale entries are replaced in-place (ON CONFLICT DO UPDATE).
//
// Tables Affected:
//   - attack_paths: new table
//
// Indexes Created:
//   - idx_attack_paths_pod_uid: fast lookup by pod
//   - idx_attack_paths_total_risk: used by unified scorer top-risk queries
//   - unique (pod_uid, path_id): upsert key
//
// Rollback Plan:
//
//	DROP TABLE IF EXISTS attack_paths;
func Migration118_AddAttackPathsTable(db *gorm.DB) error {
	log.Println("[Migration 118] Starting: add attack_paths table")

	if db.Migrator().HasTable("attack_paths") {
		log.Println("[Migration 118] ✅ attack_paths table already exists, skipping")
		return nil
	}

	createTable := `
CREATE TABLE IF NOT EXISTS attack_paths (
    id               BIGSERIAL PRIMARY KEY,
    pod_uid          VARCHAR(255) NOT NULL,
    path_id          VARCHAR(255) NOT NULL,
    nodes            JSONB        NOT NULL DEFAULT '[]',
    edges            JSONB        NOT NULL DEFAULT '[]',
    total_risk       FLOAT        NOT NULL DEFAULT 0,
    difficulty       FLOAT        NOT NULL DEFAULT 1,
    impact           FLOAT        NOT NULL DEFAULT 0,
    length           INT          NOT NULL DEFAULT 0,
    description      TEXT,
    enriched_from_pce BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_attack_paths_pod_path UNIQUE (pod_uid, path_id)
);
`
	if err := execDDL(db, createTable); err != nil {
		return err
	}
	if err := execDDL(db, `CREATE INDEX IF NOT EXISTS idx_attack_paths_pod_uid ON attack_paths(pod_uid);`); err != nil {
		return err
	}
	if err := execDDL(db, `CREATE INDEX IF NOT EXISTS idx_attack_paths_total_risk ON attack_paths(total_risk DESC);`); err != nil {
		return err
	}

	log.Println("[Migration 118] ✅ Completed - attack_paths table created")
	return nil
}
