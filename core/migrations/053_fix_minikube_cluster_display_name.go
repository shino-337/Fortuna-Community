package migrations

import (
	"log"
	"os"

	"gorm.io/gorm"
)

// Migration053_FixMinikubeClusterDisplayName updates an existing cluster row's display name
// using only environment variables. No hardcoded cluster id or name.
// Set CLUSTER_ID_TO_UPDATE = cluster id to rename (e.g. legacy id from DB).
// Set CLUSTER_DISPLAY_NAME = new display name (e.g. from kubectl config get-clusters).
// If either is unset, migration is a no-op; agent sync drives cluster name from env/kubeconfig.
func Migration053_FixMinikubeClusterDisplayName(db *gorm.DB) error {
	clusterIDToUpdate := os.Getenv("CLUSTER_ID_TO_UPDATE")
	targetName := os.Getenv("CLUSTER_DISPLAY_NAME")
	if clusterIDToUpdate == "" || targetName == "" {
		log.Println("[Migration 053] CLUSTER_ID_TO_UPDATE or CLUSTER_DISPLAY_NAME not set, skipping (use env from actual environment)")
		return nil
	}

	log.Printf("[Migration 053] Update cluster display name: id=%q -> name=%q (from env)", clusterIDToUpdate, targetName)

	if !db.Migrator().HasTable("clusters") {
		log.Println("[Migration 053] clusters table does not exist, skipping")
		return nil
	}

	result := db.Exec(
		"UPDATE clusters SET name = ? WHERE id = ? AND deleted_at IS NULL",
		targetName, clusterIDToUpdate,
	)
	if result.Error != nil {
		log.Printf("[Migration 053] ⚠️  Error updating cluster name: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("[Migration 053] ✅ Updated %d cluster row(s) to display name %q", result.RowsAffected, targetName)
	} else {
		log.Printf("[Migration 053] No cluster with id=%q found (nothing to update)", clusterIDToUpdate)
	}
	return nil
}
