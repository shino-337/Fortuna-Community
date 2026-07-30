package api

import (
	"log"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// CanonicalClusterIDPrefix is the required prefix for cluster_id from agent discovery (kube-system UID hash).
// Agent must send cluster_id from cluster.Discover (sha256-<hex>) or env CLUSTER_ID; never use display name.
const CanonicalClusterIDPrefix = "sha256-"

// IsCanonicalClusterID returns true if id is in canonical form (from agent discovery or env).
func IsCanonicalClusterID(id string) bool {
	return strings.HasPrefix(id, CanonicalClusterIDPrefix)
}

// NormalizeClusterID resolves the cluster_id used for sync to a single canonical id to avoid duplicates.
// If the received id is already canonical (sha256-...), it is returned as-is.
// If the received id looks like a legacy display name (e.g. "kubernetes"), we look up an existing cluster
// with that name and canonical id; if found, we return that id so sync is merged into the same cluster.
// Otherwise the received id is returned unchanged (e.g. custom env CLUSTER_ID).
func NormalizeClusterID(db *gorm.DB, receivedID string) string {
	if receivedID == "" {
		return receivedID
	}
	if IsCanonicalClusterID(receivedID) {
		return receivedID
	}
	var existing models.Cluster
	err := db.Where("name = ? AND id LIKE ?", receivedID, CanonicalClusterIDPrefix+"%").First(&existing).Error
	if err == nil && existing.ID != "" {
		log.Printf("[AgentAPI] cluster_id normalized: %q → %q (use canonical id for sync)", receivedID, existing.ID)
		return existing.ID
	}
	return receivedID
}
