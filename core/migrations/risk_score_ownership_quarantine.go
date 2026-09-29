package migrations

import (
	"fmt"
	"gorm.io/gorm"
)

// Risk scores use a snapshot upsert key, not a history key. Assigning an old
// unowned row to an occupied key would overwrite or discard evidence. Retain
// every colliding row unowned and record its complete original payload instead.
func quarantineRiskScoreOwnershipCollisions(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" || !db.Migrator().HasTable("risk_scores") {
		return nil
	}
	for _, column := range []string{"id", "resource_type", "resource_uid", "cluster_id"} {
		if !db.Migrator().HasColumn("risk_scores", column) {
			return fmt.Errorf("risk ownership quarantine: missing %s", column)
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS risk_score_ownership_quarantines (risk_score_id bigint PRIMARY KEY, proposed_cluster_id text NOT NULL, reason text NOT NULL, evidence jsonb NOT NULL, recorded_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP)`).Error; err != nil {
			return err
		}
		// Snapshot writers can run concurrently during a rehearsal. Lock the table
		// while recording collision evidence; the subsequent backfill is protected
		// by the existing unique key and still fails closed for any later race.
		if err := tx.Exec(`LOCK TABLE risk_scores IN SHARE ROW EXCLUSIVE MODE`).Error; err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO risk_score_ownership_quarantines (risk_score_id, proposed_cluster_id, reason, evidence)
    SELECT old.id, owners.cluster_id, 'occupied_snapshot_identity', to_jsonb(old)
    FROM risk_scores old
    JOIN (SELECT uid, MIN(cluster_id) AS cluster_id FROM pods WHERE COALESCE(cluster_id,'') <> '' GROUP BY uid HAVING COUNT(DISTINCT cluster_id) = 1) owners ON owners.uid = old.resource_uid
    WHERE COALESCE(old.cluster_id,'') = '' AND LOWER(old.resource_type) = 'pod'
      AND EXISTS (SELECT 1 FROM risk_scores other WHERE other.id <> old.id AND other.resource_uid = old.resource_uid AND other.resource_type = old.resource_type AND (other.cluster_id = owners.cluster_id OR COALESCE(other.cluster_id,'') = ''))
    ON CONFLICT (risk_score_id) DO NOTHING`).Error
	})
}
