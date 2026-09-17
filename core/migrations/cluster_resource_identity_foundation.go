package migrations

import (
	"fmt"

	"gorm.io/gorm"
)

// clusterOwnedPodTable describes persisted workload state that must retain the
// cluster half of the canonical {cluster_id, pod_uid} identity. These are static
// table/column names, never request-controlled identifiers.
type clusterOwnedPodTable struct {
	table     string
	uidColumn string
	extra     string
}

var clusterOwnedPodTables = []clusterOwnedPodTable{
	{table: "asset_security_state", uidColumn: "pod_uid"},
	{table: "attack_paths", uidColumn: "pod_uid"},
	{table: "cve_matches", uidColumn: "pod_uid"},
	{table: "events_index", uidColumn: "pod_uid"},
	{table: "exception_policies", uidColumn: "resource_uid"},
	{table: "malware_matches", uidColumn: "pod_uid"},
	{table: "pod_attack_steps", uidColumn: "pod_uid"},
	{table: "pod_capabilities", uidColumn: "pod_uid"},
	{table: "pod_image_scans", uidColumn: "pod_uid"},
	{table: "pod_instances", uidColumn: "pod_uid"},
	{table: "pod_network_connections", uidColumn: "pod_uid"},
	{table: "pod_processes", uidColumn: "pod_uid"},
	{table: "pod_risk_profiles", uidColumn: "pod_uid"},
	{table: "pod_runtime_metrics", uidColumn: "pod_uid"},
	{table: "runtime_behavior_facts", uidColumn: "pod_uid"},
	{table: "runtime_events", uidColumn: "pod_uid"},
	{table: "runtime_incidents", uidColumn: "pod_uid"},
	{table: "runtime_signals", uidColumn: "pod_uid"},
	{table: "sboms", uidColumn: "pod_uid"},
	{table: "insights", uidColumn: "resource_uid", extra: "LOWER(resource_type) = 'pod'"},
	{table: "risk_scores", uidColumn: "resource_uid", extra: "LOWER(resource_type) = 'pod'"},
	{table: "policy_violations", uidColumn: "resource_uid", extra: "LOWER(resource_type) = 'pod'"},
}

// EnsureClusterResourceIdentityFoundation is an idempotent, fail-closed schema
// invariant run after the legacy positional migration runner. It exists outside
// the positional schema_migrations list deliberately: historical migration
// versions are array indexes, so inserting/reordering a migration would corrupt
// already-applied databases.
func EnsureClusterResourceIdentityFoundation(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("cluster resource identity foundation: database is nil")
	}
	if err := validateAuthoritativePodSchema(db); err != nil {
		return err
	}
	if err := ensureAgentCompositeIdentity(db); err != nil {
		return err
	}
	if err := ensureClusterResourceIdentityColumns(db); err != nil {
		return err
	}
	if err := backfillUnambiguousPodClusterOwnership(db); err != nil {
		return err
	}
	if err := ensureClusterResourceIdentityIndexes(db); err != nil {
		return err
	}
	if err := validateExistingClusterResourceOwnership(db); err != nil {
		return err
	}
	return nil
}

func validateAuthoritativePodSchema(db *gorm.DB) error {
	if !db.Migrator().HasTable("pods") {
		return fmt.Errorf("cluster resource identity foundation: pods table is missing")
	}
	for _, column := range []string{"uid", "cluster_id"} {
		if !db.Migrator().HasColumn("pods", column) {
			return fmt.Errorf("cluster resource identity foundation: pods.%s is missing", column)
		}
	}
	return nil
}

// ensureAgentCompositeIdentity removes the legacy global AgentID uniqueness and
// replaces it with {cluster_id,agent_id}. This permits identical node/Agent names
// in independent clusters while preserving one Agent row per cluster identity.
func ensureAgentCompositeIdentity(db *gorm.DB) error {
	if !db.Migrator().HasTable("agents") {
		return fmt.Errorf("cluster resource identity foundation: required table agents is missing")
	}
	for _, column := range []string{"cluster_id", "agent_id"} {
		if !db.Migrator().HasColumn("agents", column) {
			return fmt.Errorf("cluster resource identity foundation: required column agents.%s is missing", column)
		}
	}
	if err := db.Exec("DROP INDEX IF EXISTS idx_agents_agent_id").Error; err != nil {
		return fmt.Errorf("drop legacy global agent identity index: %w", err)
	}
	if err := ensureIndex(db, "idx_agents_cluster_agent", "agents", "cluster_id, agent_id", true); err != nil {
		return err
	}

	// A legacy empty-cluster row and an exact composite row for the same AgentID
	// cannot be safely merged automatically. Refuse startup instead of letting a
	// later control RPC hit a uniqueness race or silently choose one identity.
	var ambiguous int64
	if err := db.Raw(`SELECT COUNT(*)
FROM agents legacy
WHERE COALESCE(legacy.cluster_id, '') = ''
  AND EXISTS (
    SELECT 1 FROM agents exact
    WHERE exact.agent_id = legacy.agent_id
      AND COALESCE(exact.cluster_id, '') <> ''
  )`).Scan(&ambiguous).Error; err != nil {
		return fmt.Errorf("validate legacy agent identity state: %w", err)
	}
	if ambiguous > 0 {
		return fmt.Errorf("cluster resource identity foundation: agents contains %d ambiguous legacy row(s) that coexist with cluster-qualified identities", ambiguous)
	}
	return nil
}

func ensureClusterResourceIdentityColumns(db *gorm.DB) error {
	for _, target := range clusterOwnedPodTables {
		if !db.Migrator().HasTable(target.table) {
			return fmt.Errorf("cluster resource identity foundation: required table %s is missing", target.table)
		}
		if !db.Migrator().HasColumn(target.table, target.uidColumn) {
			return fmt.Errorf("cluster resource identity foundation: required column %s.%s is missing", target.table, target.uidColumn)
		}
		if !db.Migrator().HasColumn(target.table, "cluster_id") {
			if err := db.Exec("ALTER TABLE " + target.table + " ADD COLUMN cluster_id VARCHAR(255)").Error; err != nil {
				return fmt.Errorf("add %s.cluster_id: %w", target.table, err)
			}
			if !db.Migrator().HasColumn(target.table, "cluster_id") {
				return fmt.Errorf("cluster resource identity foundation: %s.cluster_id is still missing after ALTER TABLE", target.table)
			}
		}
	}
	return nil
}

// backfillUnambiguousPodClusterOwnership performs one set-based UPDATE per target
// table. Ownership is copied only when the authoritative pods table maps a UID to
// exactly one distinct cluster. Ambiguous duplicate UIDs remain unresolved.
func backfillUnambiguousPodClusterOwnership(db *gorm.DB) error {
	for _, target := range clusterOwnedPodTables {
		stmt := fmt.Sprintf(`UPDATE %s
SET cluster_id = (
	SELECT MIN(p.cluster_id)
	FROM pods p
	WHERE p.uid = %s.%s
	  AND COALESCE(p.uid, '') <> ''
	  AND COALESCE(p.cluster_id, '') <> ''
	GROUP BY p.uid
	HAVING COUNT(DISTINCT p.cluster_id) = 1
)
WHERE COALESCE(cluster_id, '') = ''
  AND COALESCE(%s, '') <> ''`, target.table, target.table, target.uidColumn, target.uidColumn)
		if target.extra != "" {
			stmt += "\n  AND (" + target.extra + ")"
		}
		stmt += fmt.Sprintf(`
  AND (
	SELECT COUNT(DISTINCT p.cluster_id)
	FROM pods p
	WHERE p.uid = %s.%s
	  AND COALESCE(p.cluster_id, '') <> ''
  ) = 1`, target.table, target.uidColumn)

		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("backfill %s cluster ownership: %w", target.table, err)
		}
	}
	return nil
}

// validateExistingClusterResourceOwnership rejects already-populated cluster IDs
// that do not correspond to an authoritative {cluster_id,pod_uid} pair. Existing
// non-empty ownership is never silently rewritten.
func validateExistingClusterResourceOwnership(db *gorm.DB) error {
	for _, target := range clusterOwnedPodTables {
		stmt := fmt.Sprintf(`SELECT COUNT(*)
FROM %s t
WHERE COALESCE(t.cluster_id, '') <> ''
  AND COALESCE(t.%s, '') <> ''`, target.table, target.uidColumn)
		if target.extra != "" {
			stmt += "\n  AND (" + target.extra + ")"
		}
		stmt += fmt.Sprintf(`
  AND NOT EXISTS (
	SELECT 1
	FROM pods p
	WHERE p.uid = t.%s
	  AND p.cluster_id = t.cluster_id
  )`, target.uidColumn)

		var inconsistent int64
		if err := db.Raw(stmt).Scan(&inconsistent).Error; err != nil {
			return fmt.Errorf("validate %s cluster ownership: %w", target.table, err)
		}
		if inconsistent > 0 {
			return fmt.Errorf("cluster resource identity foundation: %s contains %d row(s) with cluster ownership inconsistent with pods", target.table, inconsistent)
		}
	}
	return nil
}

func ensureClusterResourceIdentityIndexes(db *gorm.DB) error {
	for _, target := range clusterOwnedPodTables {
		name := "idx_" + target.table + "_cluster_" + target.uidColumn
		if err := ensureIndex(db, name, target.table, "cluster_id, "+target.uidColumn, false); err != nil {
			return err
		}
	}
	return nil
}

// ensureIndex creates an index and, on PostgreSQL, verifies pg_index.indisvalid.
// CREATE INDEX CONCURRENTLY can leave an INVALID object after interruption; an
// IF NOT EXISTS retry would otherwise silently accept it forever.
func ensureIndex(db *gorm.DB, name, table, columns string, unique bool) error {
	uniqueSQL := ""
	if unique {
		uniqueSQL = "UNIQUE "
	}
	if db.Dialector.Name() != "postgres" {
		stmt := "CREATE " + uniqueSQL + "INDEX IF NOT EXISTS " + name + " ON " + table + " (" + columns + ")"
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
		return nil
	}

	valid, exists, err := postgresIndexValidity(db, name)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", name, err)
	}
	if exists && !valid {
		if err := db.Exec("DROP INDEX CONCURRENTLY IF EXISTS " + name).Error; err != nil {
			return fmt.Errorf("drop invalid %s: %w", name, err)
		}
		exists = false
	}
	if !exists {
		stmt := "CREATE " + uniqueSQL + "INDEX CONCURRENTLY " + name + " ON " + table + " (" + columns + ")"
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
	}
	valid, exists, err = postgresIndexValidity(db, name)
	if err != nil {
		return fmt.Errorf("verify %s: %w", name, err)
	}
	if !exists || !valid {
		return fmt.Errorf("cluster resource identity foundation: required PostgreSQL index %s is missing or invalid", name)
	}
	return nil
}

func postgresIndexValidity(db *gorm.DB, name string) (valid bool, exists bool, err error) {
	var row struct {
		Valid bool
	}
	res := db.Raw(`SELECT i.indisvalid AS valid
FROM pg_index i
JOIN pg_class c ON c.oid = i.indexrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relname = ?
  AND n.nspname = current_schema()`, name).Scan(&row)
	if res.Error != nil {
		return false, false, res.Error
	}
	if res.RowsAffected == 0 {
		return false, false, nil
	}
	return row.Valid, true, nil
}
