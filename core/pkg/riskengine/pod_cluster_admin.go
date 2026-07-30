package riskengine

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

type roleRefMinimal struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type subjectMinimal struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// clusterAdminBindingForPod returns true when a RoleBinding or ClusterRoleBinding in DB grants
// the cluster ClusterRole "cluster-admin" to the pod's ServiceAccount (same logic as GetPodRiskReport).
func clusterAdminBindingForPod(ctx context.Context, db *gorm.DB, clusterID, podNamespace, serviceAccount string) bool {
	if db == nil || strings.TrimSpace(clusterID) == "" || strings.TrimSpace(podNamespace) == "" || strings.TrimSpace(serviceAccount) == "" {
		return false
	}

	var rbs []models.RoleBinding
	// Ignore query errors (e.g. tests without migrated bindings tables); treat as no bindings.
	_ = db.WithContext(ctx).Where("cluster_id = ? AND namespace = ? AND deleted_at IS NULL", clusterID, podNamespace).Find(&rbs).Error
	for _, rb := range rbs {
		if bindingGrantsClusterAdminToSA(rb.Subjects, rb.RoleRef, podNamespace, serviceAccount) {
			return true
		}
	}

	var crbs []models.ClusterRoleBinding
	_ = db.WithContext(ctx).Where("cluster_id = ? AND deleted_at IS NULL", clusterID).Find(&crbs).Error
	for _, crb := range crbs {
		if bindingGrantsClusterAdminToSA(crb.Subjects, crb.RoleRef, podNamespace, serviceAccount) {
			return true
		}
	}
	return false
}

func bindingGrantsClusterAdminToSA(subjectsJSON, roleRefJSON, saNamespace, saName string) bool {
	var subjects []subjectMinimal
	if err := json.Unmarshal([]byte(subjectsJSON), &subjects); err != nil {
		return false
	}
	matched := false
	for _, sub := range subjects {
		if sub.Kind == "ServiceAccount" && sub.Name == saName && sub.Namespace == saNamespace {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	var ref roleRefMinimal
	if err := json.Unmarshal([]byte(roleRefJSON), &ref); err != nil {
		return false
	}
	if !strings.EqualFold(ref.Kind, "ClusterRole") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(ref.Name), "cluster-admin")
}
