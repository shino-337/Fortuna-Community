package migrations

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type clusterQualifiedUniqueIndex struct {
	name              string
	table             string
	columns           string
	legacyIndexes     []string
	legacyConstraints []string
}

type clusterQualifiedPrimaryKey struct {
	table      string
	columns    string
	constraint string
	guardIndex string
}

var clusterQualifiedPodUniqueIndexes = []clusterQualifiedUniqueIndex{
	{
		name:          "idx_pod_capability_identity",
		table:         "pod_capabilities",
		columns:       "cluster_id, pod_uid, capability_id",
		legacyIndexes: []string{"idx_pod_capabilities_unique"},
	},
	{
		name:          "idx_pod_risk_profile_identity",
		table:         "pod_risk_profiles",
		columns:       "cluster_id, pod_uid",
		legacyIndexes: []string{"idx_pod_risk_profiles_unique"},
	},
	{
		name:              "idx_attack_path_identity",
		table:             "attack_paths",
		columns:           "cluster_id, pod_uid, path_id",
		legacyConstraints: []string{"uq_attack_paths_pod_path"},
	},
	{
		name:      "idx_pod_image_scan_identity",
		table:     "pod_image_scans",
		columns:   "cluster_id, pod_uid, container_name",
		legacyIndexes: []string{
			"idx_pod_image_scans_unique_pod_uid_container_name",
			"idx_pod_image_scans_unique_pod_uid_container_name_all",
		},
	},
}

var clusterQualifiedPodPrimaryKeys = []clusterQualifiedPrimaryKey{
	{
		table:      "pod_attack_steps",
		columns:    "cluster_id, pod_uid, step_id",
		constraint: "pod_attack_steps_pkey",
		guardIndex: "idx_pod_attack_step_identity_guard",
	},
	{
		table:      "pod_instances",
		columns:    "cluster_id, pod_uid",
		constraint: "pod_instances_pkey",
		guardIndex: "idx_pod_instance_identity_guard",
	},
}

// EnsureClusterQualifiedPodUniqueness removes database-level assumptions that a
// Kubernetes Pod UID is globally unique. It runs after the ownership foundation
// has backfilled cluster_id. Replacement composite uniqueness is established and
// verified before any legacy UID-only key is removed.
func EnsureClusterQualifiedPodUniqueness(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("cluster-qualified pod uniqueness: database is nil")
	}

	for _, target := range clusterQualifiedPodUniqueIndexes {
		if err := validateClusterQualifiedKeyTarget(db, target.table, target.columns); err != nil {
			return err
		}
		if err := rejectUnownedRowsForCanonicalKey(db, target.table); err != nil {
			return err
		}
		if err := ensureIndex(db, target.name, target.table, target.columns, true); err != nil {
			return fmt.Errorf("ensure %s: %w", target.name, err)
		}
	}

	for _, target := range clusterQualifiedPodPrimaryKeys {
		if err := validateClusterQualifiedKeyTarget(db, target.table, target.columns); err != nil {
			return err
		}
		if err := rejectUnownedRowsForCanonicalKey(db, target.table); err != nil {
			return err
		}
		// A temporary composite unique guard preserves strictness while PostgreSQL
		// swaps the old UID-only primary key.
		if err := ensureIndex(db, target.guardIndex, target.table, target.columns, true); err != nil {
			return fmt.Errorf("ensure primary-key guard %s: %w", target.guardIndex, err)
		}
	}

	if db.Dialector.Name() == "postgres" {
		for _, target := range clusterQualifiedPodUniqueIndexes {
			for _, constraint := range target.legacyConstraints {
				if err := db.Exec("ALTER TABLE " + target.table + " DROP CONSTRAINT IF EXISTS " + constraint).Error; err != nil {
					return fmt.Errorf("drop legacy constraint %s.%s: %w", target.table, constraint, err)
				}
			}
			for _, index := range target.legacyIndexes {
				if err := db.Exec("DROP INDEX CONCURRENTLY IF EXISTS " + index).Error; err != nil {
					return fmt.Errorf("drop legacy index %s: %w", index, err)
				}
			}
		}

		for _, target := range clusterQualifiedPodPrimaryKeys {
			if err := ensurePostgresCompositePrimaryKey(db, target); err != nil {
				return err
			}
			// Once the new PK is verified the temporary guard is redundant.
			if err := db.Exec("DROP INDEX CONCURRENTLY IF EXISTS " + target.guardIndex).Error; err != nil {
				return fmt.Errorf("drop primary-key guard %s: %w", target.guardIndex, err)
			}
		}
	} else {
		// Non-PostgreSQL CI/fresh databases receive composite model keys via
		// AutoMigrate. For legacy ordinary unique indexes, remove UID-only indexes
		// only where the dialect supports standalone DROP INDEX.
		for _, target := range clusterQualifiedPodUniqueIndexes {
			for _, index := range target.legacyIndexes {
				if err := db.Exec("DROP INDEX IF EXISTS " + index).Error; err != nil {
					return fmt.Errorf("drop legacy index %s: %w", index, err)
				}
			}
		}
	}

	return verifyClusterQualifiedPodUniqueness(db)
}

func validateClusterQualifiedKeyTarget(db *gorm.DB, table, columns string) error {
	if !db.Migrator().HasTable(table) {
		return fmt.Errorf("cluster-qualified pod uniqueness: required table %s is missing", table)
	}
	for _, column := range strings.Split(columns, ",") {
		column = strings.TrimSpace(column)
		if column == "" || !db.Migrator().HasColumn(table, column) {
			return fmt.Errorf("cluster-qualified pod uniqueness: required column %s.%s is missing", table, column)
		}
	}
	return nil
}

func rejectUnownedRowsForCanonicalKey(db *gorm.DB, table string) error {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE COALESCE(cluster_id, '') = ''").Scan(&count).Error; err != nil {
		return fmt.Errorf("validate %s cluster ownership completeness: %w", table, err)
	}
	if count != 0 {
		return fmt.Errorf("cluster-qualified pod uniqueness: %s contains %d row(s) without resolvable cluster ownership", table, count)
	}
	return nil
}

func ensurePostgresCompositePrimaryKey(db *gorm.DB, target clusterQualifiedPrimaryKey) error {
	columns, exists, err := postgresPrimaryKeyColumns(db, target.table)
	if err != nil {
		return fmt.Errorf("inspect %s primary key: %w", target.table, err)
	}
	expected := normalizeIndexColumns(target.columns)
	if exists && columns == expected {
		return nil
	}
	if exists {
		if err := db.Exec("ALTER TABLE " + target.table + " DROP CONSTRAINT " + target.constraint).Error; err != nil {
			return fmt.Errorf("drop legacy primary key %s: %w", target.constraint, err)
		}
	}
	stmt := "ALTER TABLE " + target.table + " ADD CONSTRAINT " + target.constraint + " PRIMARY KEY (" + target.columns + ")"
	if err := db.Exec(stmt).Error; err != nil {
		return fmt.Errorf("create composite primary key %s: %w", target.constraint, err)
	}
	columns, exists, err = postgresPrimaryKeyColumns(db, target.table)
	if err != nil {
		return fmt.Errorf("verify %s primary key: %w", target.table, err)
	}
	if !exists || columns != expected {
		return fmt.Errorf("cluster-qualified pod uniqueness: %s primary key has columns %q, want %q", target.table, columns, expected)
	}
	return nil
}

func postgresPrimaryKeyColumns(db *gorm.DB, table string) (string, bool, error) {
	var row struct {
		Columns string `gorm:"column:column_names"`
	}
	res := db.Raw(`SELECT COALESCE(string_agg(a.attname::text, ',' ORDER BY keys.ordinality), '') AS column_names
FROM pg_constraint c
JOIN pg_class t ON t.oid = c.conrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
JOIN LATERAL unnest(c.conkey) WITH ORDINALITY AS keys(attnum, ordinality) ON TRUE
JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = keys.attnum
WHERE c.contype = 'p'
  AND t.relname = ?
  AND n.nspname = current_schema()
GROUP BY c.oid`, table).Scan(&row)
	if res.Error != nil {
		return "", false, res.Error
	}
	if res.RowsAffected == 0 {
		return "", false, nil
	}
	return row.Columns, true, nil
}

func verifyClusterQualifiedPodUniqueness(db *gorm.DB) error {
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, target := range clusterQualifiedPodUniqueIndexes {
		def, exists, err := postgresIndexDefinitionForName(db, target.name)
		if err != nil {
			return fmt.Errorf("verify %s: %w", target.name, err)
		}
		if !exists || !def.Valid || !def.Unique || def.Table != target.table || def.Columns != normalizeIndexColumns(target.columns) {
			return fmt.Errorf("cluster-qualified pod uniqueness: %s has wrong definition", target.name)
		}
		for _, legacy := range target.legacyIndexes {
			if _, exists, err := postgresIndexDefinitionForName(db, legacy); err != nil {
				return fmt.Errorf("verify legacy index %s removal: %w", legacy, err)
			} else if exists {
				return fmt.Errorf("cluster-qualified pod uniqueness: legacy global index %s still exists", legacy)
			}
		}
	}
	for _, target := range clusterQualifiedPodPrimaryKeys {
		columns, exists, err := postgresPrimaryKeyColumns(db, target.table)
		if err != nil {
			return err
		}
		if !exists || columns != normalizeIndexColumns(target.columns) {
			return fmt.Errorf("cluster-qualified pod uniqueness: %s primary key has wrong definition", target.table)
		}
	}
	return nil
}
