package migrations

import (
	"fmt"
	"sort"

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
// The foundation is intentionally staged. It adds/backfills ClusterID and useful
// indexes, but does not yet make the new columns NOT NULL or replace legacy
// primary/unique keys. C3c query migration must move all readers/writers to
// {cluster_id, pod_uid} before those constraints are tightened.
func EnsureClusterResourceIdentityFoundation(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("cluster resource identity foundation: database is nil")
	}
	if !db.Migrator().HasTable("pods") {
		return fmt.Errorf("cluster resource identity foundation: pods table is missing")
	}

	for _, target := range clusterOwnedPodTables {
		if !db.Migrator().HasTable(target.table) {
			continue
		}
		if !db.Migrator().HasColumn(target.table, "cluster_id") {
			if err := db.Exec("ALTER TABLE " + target.table + " ADD COLUMN cluster_id VARCHAR(255)").Error; err != nil {
				return fmt.Errorf("add %s.cluster_id: %w", target.table, err)
			}
		}
	}

	if err := backfillUnambiguousPodClusterOwnership(db); err != nil {
		return err
	}
	if err := ensureClusterResourceIdentityIndexes(db); err != nil {
		return err
	}
	return nil
}

type podClusterOwner struct {
	UID       string `gorm:"column:uid"`
	ClusterID string `gorm:"column:cluster_id"`
}

func backfillUnambiguousPodClusterOwnership(db *gorm.DB) error {
	var owners []podClusterOwner
	if err := db.Unscoped().Table("pods").
		Select("uid, MIN(cluster_id) AS cluster_id").
		Where("uid <> '' AND cluster_id <> ''").
		Group("uid").
		Having("COUNT(DISTINCT cluster_id) = 1").
		Scan(&owners).Error; err != nil {
		return fmt.Errorf("resolve unambiguous pod ownership: %w", err)
	}

	byCluster := make(map[string][]string)
	for _, owner := range owners {
		if owner.ClusterID == "" || owner.UID == "" {
			continue
		}
		byCluster[owner.ClusterID] = append(byCluster[owner.ClusterID], owner.UID)
	}
	clusters := make([]string, 0, len(byCluster))
	for clusterID := range byCluster {
		clusters = append(clusters, clusterID)
	}
	sort.Strings(clusters)

	for _, target := range clusterOwnedPodTables {
		if !db.Migrator().HasTable(target.table) ||
			!db.Migrator().HasColumn(target.table, "cluster_id") ||
			!db.Migrator().HasColumn(target.table, target.uidColumn) {
			continue
		}
		for _, clusterID := range clusters {
			uids := byCluster[clusterID]
			q := db.Table(target.table).
				Where(target.uidColumn+" IN ?", uids).
				Where("COALESCE(cluster_id, '') = ''")
			if target.extra != "" {
				q = q.Where(target.extra)
			}
			if err := q.Update("cluster_id", clusterID).Error; err != nil {
				return fmt.Errorf("backfill %s cluster ownership: %w", target.table, err)
			}
		}
	}
	return nil
}

func ensureClusterResourceIdentityIndexes(db *gorm.DB) error {
	for _, target := range clusterOwnedPodTables {
		if !db.Migrator().HasTable(target.table) ||
			!db.Migrator().HasColumn(target.table, "cluster_id") ||
			!db.Migrator().HasColumn(target.table, target.uidColumn) {
			continue
		}
		name := "idx_" + target.table + "_cluster_" + target.uidColumn
		stmt := "CREATE INDEX IF NOT EXISTS " + name + " ON " + target.table + " (cluster_id, " + target.uidColumn + ")"
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
	}
	return nil
}
