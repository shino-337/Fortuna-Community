package migrations

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type clusterQualifiedUniqueIndex struct {
	name       string
	table      string
	columns    string
	legacyName string
}

var clusterQualifiedPodUniqueIndexes = []clusterQualifiedUniqueIndex{
	{
		name:       "idx_pod_capability_identity",
		table:      "pod_capabilities",
		columns:    "cluster_id, pod_uid, capability_id",
		legacyName: "idx_pod_capabilities_unique",
	},
	{
		name:       "idx_pod_risk_profile_identity",
		table:      "pod_risk_profiles",
		columns:    "cluster_id, pod_uid",
		legacyName: "idx_pod_risk_profiles_unique",
	},
}

// EnsureClusterQualifiedPodUniqueness switches writer conflict keys only after
// the cluster ownership foundation has populated/validated cluster_id. New
// composite unique indexes are created and verified before the legacy global-UID
// indexes are removed, so a failed startup never leaves the schema less strict.
func EnsureClusterQualifiedPodUniqueness(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("cluster-qualified pod uniqueness: database is nil")
	}

	for _, target := range clusterQualifiedPodUniqueIndexes {
		if !db.Migrator().HasTable(target.table) {
			return fmt.Errorf("cluster-qualified pod uniqueness: required table %s is missing", target.table)
		}
		for _, column := range strings.Split(target.columns, ",") {
			column = strings.TrimSpace(column)
			if column == "" || !db.Migrator().HasColumn(target.table, column) {
				return fmt.Errorf("cluster-qualified pod uniqueness: required column %s.%s is missing", target.table, column)
			}
		}
		if err := ensureIndex(db, target.name, target.table, target.columns, true); err != nil {
			return fmt.Errorf("ensure %s: %w", target.name, err)
		}
	}

	// Drop global-UID uniqueness only after all replacement indexes exist.
	for _, target := range clusterQualifiedPodUniqueIndexes {
		stmt := "DROP INDEX IF EXISTS " + target.legacyName
		if db.Dialector.Name() == "postgres" {
			stmt = "DROP INDEX CONCURRENTLY IF EXISTS " + target.legacyName
		}
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("drop legacy index %s: %w", target.legacyName, err)
		}
	}

	// PostgreSQL gets an exact post-condition check rather than trusting names.
	if db.Dialector.Name() == "postgres" {
		for _, target := range clusterQualifiedPodUniqueIndexes {
			def, exists, err := postgresIndexDefinitionForName(db, target.name)
			if err != nil {
				return fmt.Errorf("verify %s: %w", target.name, err)
			}
			if !exists || !def.Valid || !def.Unique || def.Table != target.table || def.Columns != normalizeIndexColumns(target.columns) {
				return fmt.Errorf("cluster-qualified pod uniqueness: %s has wrong definition", target.name)
			}
			if _, exists, err := postgresIndexDefinitionForName(db, target.legacyName); err != nil {
				return fmt.Errorf("verify legacy index %s removal: %w", target.legacyName, err)
			} else if exists {
				return fmt.Errorf("cluster-qualified pod uniqueness: legacy global index %s still exists", target.legacyName)
			}
		}
	}
	return nil
}
