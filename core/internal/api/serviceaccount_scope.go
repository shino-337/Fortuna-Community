package api

import (
	"encoding/json"
	"net/http"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func loadScopedServiceAccountByUID(db *gorm.DB, c *gin.Context, uid string, preloadCluster bool) (*models.ServiceAccount, bool) {
	scope, ok := resolveRiskGovernanceScope(db, c)
	if !ok {
		return nil, false
	}

	query := scope.apply(db.WithContext(c.Request.Context()).Model(&models.ServiceAccount{}), "cluster_id").
		Where("uid = ?", uid).
		Order("cluster_id ASC, id ASC").
		Limit(2)
	if preloadCluster {
		query = query.Preload("Cluster")
	}

	var matches []models.ServiceAccount
	if err := query.Find(&matches).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to load ServiceAccount"})
		return nil, false
	}
	if len(matches) == 0 {
		// Preserve the established scoped-reader behavior for an unqualified UID that
		// exists only outside the caller's cluster allow-list, without using the
		// foreign row as an authorization source for a successful response.
		if scope.restricted && scope.clusterID == "" {
			var foreign models.ServiceAccount
			if err := db.WithContext(c.Request.Context()).Where("uid = ?", uid).Order("cluster_id ASC, id ASC").First(&foreign).Error; err == nil {
				middleware.AbortClusterScopeDenied(db, c, foreign.ClusterID)
				return nil, false
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
		return nil, false
	}
	if len(matches) > 1 {
		code := "cluster_identity_required"
		message := "clusterId is required for an ambiguous ServiceAccount UID"
		if scope.clusterID != "" {
			code = "service_account_identity_ambiguous"
			message = "duplicate ServiceAccount identity exists in the selected cluster"
		}
		c.JSON(http.StatusConflict, gin.H{"error": message, "code": code})
		return nil, false
	}
	return &matches[0], true
}

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
