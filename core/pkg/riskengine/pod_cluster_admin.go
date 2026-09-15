package riskengine

import (
	"context"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/rbacinventory"
	"gorm.io/gorm"
	"strings"
)

// clusterAdminBindingForPod requires a resolved ClusterRoleBinding to cluster-admin.
// A RoleBinding to that role grants namespace-scoped permissions only.
func clusterAdminBindingForPod(ctx context.Context, db *gorm.DB, clusterID, podNamespace, serviceAccount string) (bool, error) {
	if db == nil || strings.TrimSpace(clusterID) == "" || strings.TrimSpace(podNamespace) == "" || strings.TrimSpace(serviceAccount) == "" {
		return false, nil
	}
	grants, err := rbacinventory.Resolve(db.WithContext(ctx), &models.ServiceAccount{ClusterID: clusterID, Namespace: podNamespace, Name: serviceAccount})
	if err != nil {
		return false, err
	}
	return grants.HasClusterAdminBinding(), nil
}
