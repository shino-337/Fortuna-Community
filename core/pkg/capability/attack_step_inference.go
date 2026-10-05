package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
)

// AttackStepInference generates attack steps from exploited capabilities
type AttackStepInference struct {
	db *gorm.DB
}

// resolveUniquePodIdentity is the compatibility bridge for legacy UID-only
// callers. It fails closed unless the UID maps to exactly one active cluster.
func resolveUniquePodIdentity(ctx context.Context, db *gorm.DB, podUID string) (resourceidentity.Identity, error) {
	var owners []string
	if err := db.WithContext(ctx).Model(&models.Pod{}).
		Where("uid = ? AND deleted_at IS NULL", podUID).
		Distinct().Order("cluster_id").Pluck("cluster_id", &owners).Error; err != nil {
		return resourceidentity.Identity{}, err
	}
	if len(owners) != 1 {
		return resourceidentity.Identity{}, fmt.Errorf("cluster-qualified pod identity required: uid=%s owners=%d", podUID, len(owners))
	}
	return resourceidentity.New(owners[0], podUID)
}

// updateStepEvidence updates evidence for existing step
func (asi *AttackStepInference) updateStepEvidence(ctx context.Context, step *models.PodAttackStep, cap *models.PodCapability) {
	var evidence map[string]interface{}
	if step.Evidence != "" {
		json.Unmarshal([]byte(step.Evidence), &evidence)
	} else {
		evidence = make(map[string]interface{})
	}

	// Add capability info
	evidence["capability_id"] = cap.CapabilityID
	evidence["capability_state"] = cap.State
	evidence["updated_at"] = time.Now().Format(time.RFC3339)

	evidenceJSON, _ := json.Marshal(evidence)
	step.Evidence = string(evidenceJSON)
}

// getStepCategory returns category for attack step ID
func getStepCategory(stepID string) string {
	categories := map[string]string{
		"NODE_FS_WRITE":         "FILESYSTEM",
		"NODE_KERNEL_ACCESS":    "KERNEL",
		"PROC_NAMESPACE_ACCESS": "PROCESS",
		"IPC_NAMESPACE_ACCESS":  "IPC",
		"NODE_CRED_DUMP":        "CREDENTIALS",
		"NODE_PERSISTENCE":      "PERSISTENCE",
		"KUBELET_CRED_ACCESS":   "CREDENTIALS",
		"PROC_ROOT_PIVOT":       "ESCAPE",
		"RBAC_ABUSE":            "RBAC",
		"NETWORK_SNIFFING":      "NETWORK",
		"RBAC_ESCALATION":       "RBAC",
		"RESOURCE_MANIPULATION": "RBAC",
		"CONTROL_PLANE_ACCESS":  "CONTROL_PLANE",
	}

	if cat, ok := categories[stepID]; ok {
		return cat
	}
	return "UNKNOWN"
}

// getStepDescription returns description for attack step ID
func getStepDescription(stepID string) string {
	descriptions := map[string]string{
		"NODE_FS_WRITE":         "Write access to node filesystem",
		"NODE_KERNEL_ACCESS":    "Access to kernel resources",
		"PROC_NAMESPACE_ACCESS": "Access to process namespace",
		"IPC_NAMESPACE_ACCESS":  "Access to IPC namespace",
		"NODE_CRED_DUMP":        "Dump credentials from node",
		"NODE_PERSISTENCE":      "Establish persistence on node",
		"KUBELET_CRED_ACCESS":   "Access kubelet credentials",
		"PROC_ROOT_PIVOT":       "Proc root pivot (container escape)",
		"RBAC_ABUSE":            "Abuse RBAC permissions",
		"NETWORK_SNIFFING":      "Sniff network traffic",
		"RBAC_ESCALATION":       "Escalate RBAC privileges",
		"RESOURCE_MANIPULATION": "Manipulate Kubernetes resources",
		"CONTROL_PLANE_ACCESS":  "Access control plane components",
	}

	if desc, ok := descriptions[stepID]; ok {
		return desc
	}
	return "Attack step: " + stepID
}
