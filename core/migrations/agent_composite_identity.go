package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// EnsureAgentCompositeIdentity installs the new key before removing global ID
// uniqueness. Unassigned legacy records are retained, never guessed from a node.
func EnsureAgentCompositeIdentity(db *gorm.DB) error {
	if !db.Migrator().HasTable("agents") {
		return fmt.Errorf("agents table is missing")
	}
	if err := Migration150_AgentClusterIdentity(db); err != nil {
		return err
	}
	if err := ensureIndex(db, "idx_agents_cluster_agent", "agents", "cluster_id, agent_id", true); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_agent_id_key").Error; err != nil {
				return err
			}
		}
		if err := tx.Exec("DROP INDEX IF EXISTS idx_agents_agent_id").Error; err != nil {
			return err
		}
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec(`CREATE OR REPLACE FUNCTION fortuna_agent_identity_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' AND (COALESCE(NEW.cluster_id,'')='' OR COALESCE(NEW.agent_id,'')='') THEN RAISE EXCEPTION 'Agent cluster and ID are required'; END IF;
 IF TG_OP='UPDATE' AND (NEW.cluster_id,NEW.agent_id) IS DISTINCT FROM (OLD.cluster_id,OLD.agent_id) THEN RAISE EXCEPTION 'Agent ownership is immutable'; END IF;
 RETURN NEW;
END $$`).Error; err != nil {
				return err
			}
			if err := tx.Exec("DROP TRIGGER IF EXISTS agent_identity_guard ON agents").Error; err != nil {
				return err
			}
			return tx.Exec("CREATE TRIGGER agent_identity_guard BEFORE INSERT OR UPDATE OF cluster_id,agent_id ON agents FOR EACH ROW EXECUTE FUNCTION fortuna_agent_identity_guard()").Error
		}
		return nil
	})
}
