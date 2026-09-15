package riskengine

import (
	"context"
	"encoding/json"
	"fmt"
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
func clusterAdminBindingForPod(ctx context.Context, db *gorm.DB, clusterID, podNamespace, serviceAccount string) (bool, error) {
	if db == nil || strings.TrimSpace(clusterID) == "" || strings.TrimSpace(podNamespace) == "" || strings.TrimSpace(serviceAccount) == "" {
		return false, nil
	}

	var rbs []models.RoleBinding
	// Required evidence must be available before interpreting absence.
	if err := db.WithContext(ctx).Where("cluster_id = ? AND namespace = ? AND deleted_at IS NULL", clusterID, podNamespace).Find(&rbs).Error; err != nil {
		return false, fmt.Errorf("read role bindings: %w", err)
	}
	for _, rb := range rbs {
		var subjects []subjectMinimal
		var ref roleRefMinimal
		if json.Unmarshal([]byte(rb.Subjects), &subjects) != nil || json.Unmarshal([]byte(rb.RoleRef), &ref) != nil {
			return false, fmt.Errorf("invalid role binding JSON")
		}
		if bindingGrantsClusterAdminToSA(rb.Subjects, rb.RoleRef, podNamespace, serviceAccount) {
			return true, nil
		}
	}

	var crbs []models.ClusterRoleBinding
	if err := db.WithContext(ctx).Where("cluster_id = ? AND deleted_at IS NULL", clusterID).Find(&crbs).Error; err != nil {
		return false, fmt.Errorf("read cluster role bindings: %w", err)
	}
	for _, crb := range crbs {
		var subjects []subjectMinimal
		var ref roleRefMinimal
		if json.Unmarshal([]byte(crb.Subjects), &subjects) != nil || json.Unmarshal([]byte(crb.RoleRef), &ref) != nil {
			return false, fmt.Errorf("invalid cluster role binding JSON")
		}
		if bindingGrantsClusterAdminToSA(crb.Subjects, crb.RoleRef, podNamespace, serviceAccount) {
			return true, nil
		}
	}
	return false, nil
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
