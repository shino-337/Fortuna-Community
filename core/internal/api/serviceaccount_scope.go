package api

import (
	"encoding/json"
	"net/http"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func authorizeServiceAccount(db *gorm.DB, c *gin.Context, sa *models.ServiceAccount) bool {
	scope, ok := resolveRiskGovernanceScope(db, c)
	if !ok {
		return false
	}
	if (scope.restricted && sa.ClusterID == "") || !middleware.ClusterAllowed(c, sa.ClusterID) || (scope.clusterID != "" && scope.clusterID != sa.ClusterID) {
		middleware.AbortClusterScopeDenied(db, c, sa.ClusterID)
		return false
	}
	return true
}

// Inventory edits are local metadata only. Agent-observed identity, credentials,
// relationships and lifecycle fields must never be supplied to GORM Updates.
func serviceAccountMetadataUpdate(c *gin.Context) (map[string]interface{}, bool) {
	var fields map[string]json.RawMessage
	if err := c.ShouldBindJSON(&fields); err != nil || len(fields) != 1 || fields["labels"] == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only labels may be updated"})
		return nil, false
	}
	raw := fields["labels"]
	var encoded string
	if json.Unmarshal(raw, &encoded) == nil {
		raw = []byte(encoded)
	}
	var labels map[string]string
	if err := json.Unmarshal(raw, &labels); err != nil || labels == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "labels must be a JSON object of string values"})
		return nil, false
	}
	normalized, err := json.Marshal(labels)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid labels"})
		return nil, false
	}
	return map[string]interface{}{"labels": string(normalized)}, true
}
