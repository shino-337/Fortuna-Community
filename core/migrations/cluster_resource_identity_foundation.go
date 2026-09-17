package migrations

import (
	"fmt"
	"strings"

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
// invariant run after the legacy positional migration runner. Agent identity is
// intentionally not changed here: legacy Agent control paths still key by agent_id
// alone, so enabling duplicate AgentIDs before that trusted-principal cutover would
// create a deployment-order ambiguity.
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

// backfillUnambiguousPodClusterOwnership copies ownership only when the
// authoritative pods table maps a UID to exactly one distinct cluster.
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
// that do not correspond to an authoritative {cluster_id,pod_uid} pair.
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

type postgresIndexDefinition struct {
	Valid   bool   `gorm:"column:valid"`
	Unique  bool   `gorm:"column:is_unique"`
	Table   string `gorm:"column:table_name"`
	Columns string `gorm:"column:column_names"`
}

// ensureIndex does not trust an index merely because its name exists. On
// PostgreSQL it verifies validity, target table, ordered columns and uniqueness.
// A stale/wrong definition is dropped and rebuilt fail-closed.
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

	expectedColumns := normalizeIndexColumns(columns)
	def, exists, err := postgresIndexDefinitionForName(db, name)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", name, err)
	}
	if exists && (!def.Valid || def.Unique != unique || def.Table != table || def.Columns != expectedColumns) {
		if err := db.Exec("DROP INDEX CONCURRENTLY IF EXISTS " + name).Error; err != nil {
			return fmt.Errorf("drop invalid or mismatched %s: %w", name, err)
		}
		exists = false
	}
	if !exists {
		stmt := "CREATE " + uniqueSQL + "INDEX CONCURRENTLY " + name + " ON " + table + " (" + columns + ")"
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
	}
	def, exists, err = postgresIndexDefinitionForName(db, name)
	if err != nil {
		return fmt.Errorf("verify %s: %w", name, err)
	}
	if !exists || !def.Valid || def.Unique != unique || def.Table != table || def.Columns != expectedColumns {
		return fmt.Errorf("cluster resource identity foundation: required PostgreSQL index %s has wrong definition", name)
	}
	return nil
}

func normalizeIndexColumns(columns string) string {
	parts := strings.Split(columns, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, ",")
}

func postgresIndexDefinitionForName(db *gorm.DB, name string) (postgresIndexDefinition, bool, error) {
	var row postgresIndexDefinition
	res := db.Raw(`SELECT
  i.indisvalid AS valid,
  i.indisunique AS is_unique,
  tbl.relname::text AS table_name,
  COALESCE(string_agg(att.attname::text, ',' ORDER BY keycols.ordinality), '') AS column_names
FROM pg_index i
JOIN pg_class idx ON idx.oid = i.indexrelid
JOIN pg_class tbl ON tbl.oid = i.indrelid
JOIN pg_namespace n ON n.oid = idx.relnamespace
LEFT JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS keycols(attnum, ordinality) ON TRUE
LEFT JOIN pg_attribute att ON att.attrelid = i.indrelid AND att.attnum = keycols.attnum
WHERE idx.relname = ?
  AND n.nspname = current_schema()
GROUP BY i.indisvalid, i.indisunique, tbl.relname`, name).Scan(&row)
	if res.Error != nil {
		return postgresIndexDefinition{}, false, res.Error
	}
	if res.RowsAffected == 0 {
		return postgresIndexDefinition{}, false, nil
	}
	return row, true, nil
}

// postgresIndexValidity remains as a narrow helper for existing regression tests.
func postgresIndexValidity(db *gorm.DB, name string) (valid bool, exists bool, err error) {
	def, exists, err := postgresIndexDefinitionForName(db, name)
	if err != nil || !exists {
		return false, exists, err
	}
	return def.Valid, true, nil
}
