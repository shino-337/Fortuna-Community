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
	{table: "insights", uidColumn: "resource_uid", extra: "resource_type = 'Pod'"},
}

// EnsureClusterResourceIdentityFoundation is an idempotent, fail-closed schema
// invariant run after the legacy positional migration runner. It exists outside
// the positional schema_migrations list deliberately: historical migration
// versions are array indexes, so inserting/reordering a migration would corrupt
// already-applied databases.
//
// Every table in clusterOwnedPodTables is required at this point in startup. The
// legacy runner may continue after individual migration failures, so silently
// skipping a missing security-critical table/UID column here would turn a broken
// schema into an apparently healthy startup.
//
// The foundation is intentionally staged. It adds/backfills ClusterID and useful
// indexes, but does not yet make the new columns NOT NULL or replace legacy
// primary/unique keys. C3c query migration must move all readers/writers to
// {cluster_id, pod_uid} before those constraints are tightened.
func EnsureClusterResourceIdentityFoundation(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("cluster resource identity foundation: database is nil")
	}
	if err := validateAuthoritativePodSchema(db); err != nil {
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
// exactly one distinct cluster. This avoids the unbounded application-side UID IN
// lists used by the first implementation and keeps ambiguous UIDs unresolved.
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
// non-empty ownership is never silently rewritten because doing so would guess at
// security-sensitive historical state.
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
		prefix := "CREATE INDEX IF NOT EXISTS "
		if db.Dialector.Name() == "postgres" {
			// Startup schema enforcement is intentionally outside a transaction. Use
			// PostgreSQL's concurrent index build to avoid blocking normal table
			// writes for the duration of a large historical index build.
			prefix = "CREATE INDEX CONCURRENTLY IF NOT EXISTS "
		}
		stmt := prefix + name + " ON " + target.table + " (cluster_id, " + target.uidColumn + ")"
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
	}
	return nil
}
