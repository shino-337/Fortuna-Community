package correlator

import (
	"encoding/json"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/ksam/core/internal/normalizer"
	"github.com/ksam/core/internal/risk"
	"github.com/ksam/core/pkg/models"
)

// Correlator builds relationships between resources
type Correlator struct {
	db         *gorm.DB
	riskEngine *risk.RiskEngine
}

// NewCorrelator creates a new correlator
func NewCorrelator(db *gorm.DB) *Correlator {
	return &Correlator{
		db:         db,
		riskEngine: risk.NewRiskEngine(db),
	}
}

// Correlate correlates an inventory item with existing resources
func (c *Correlator) Correlate(item *normalizer.NormalizedInventoryItem) error {
	log.Printf("[Correlator] Correlating item: kind=%s, name=%s, namespace=%s", 
		item.Kind, item.Name, item.Namespace)
	
	// Parse raw JSON to extract cluster_id
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(item.RawJSON), &rawData); err != nil {
		log.Printf("[Correlator] Failed to parse raw JSON: %v", err)
		return nil // Continue even if parsing fails
	}
	
	// Extract metadata
	metadata, ok := rawData["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	
	// Get cluster_id from metadata labels or annotations
	clusterID := ""
	if labels, ok := metadata["labels"].(map[string]interface{}); ok {
		if cid, ok := labels["cluster_id"].(string); ok {
			clusterID = cid
		}
	}
	
	// Handle different resource types
	var err error
	switch item.Kind {
	case "Pod":
		err = c.correlatePod(item, clusterID, rawData)
	case "ServiceAccount":
		err = c.correlateServiceAccount(item, clusterID, rawData)
	case "Role", "ClusterRole":
		err = c.correlateRole(item, clusterID, rawData)
	case "RoleBinding", "ClusterRoleBinding":
		err = c.correlateRoleBinding(item, clusterID, rawData)
	default:
		// Other resource types - just log
		log.Printf("[Correlator] Unhandled resource type: %s", item.Kind)
		return nil
	}
	
	// Trigger risk evaluation after correlation for RBAC resources
	// Run async to avoid blocking correlation
	if item.Kind == "ServiceAccount" || item.Kind == "Role" || 
	   item.Kind == "ClusterRole" || item.Kind == "RoleBinding" || 
	   item.Kind == "ClusterRoleBinding" {
		go func() {
			if evalErr := c.riskEngine.EvaluateRBACRisks(); evalErr != nil {
				log.Printf("[Correlator] Risk evaluation error: %v", evalErr)
			}
		}()
	}
	
	return err
}

// correlatePod links Pod to ServiceAccount and Node
func (c *Correlator) correlatePod(item *normalizer.NormalizedInventoryItem, clusterID string, rawData map[string]interface{}) error {
	// Extract pod spec
	spec, ok := rawData["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	
	// Service account name is already stored in pod record
	
	// Get node name
	nodeName := ""
	if node, ok := spec["nodeName"].(string); ok {
		nodeName = node
	}
	
	// Find or create pod
	var pod models.Pod
	err := c.db.Where("uid = ?", item.UID).First(&pod).Error
	if err == gorm.ErrRecordNotFound {
		// Pod will be created by sync process
		log.Printf("[Correlator] Pod %s not found, will be created by sync", item.UID)
	} else if err == nil {
		// Update pod with node_id if node name is available
		if nodeName != "" {
			var node models.Node
			if err := c.db.Where("cluster_id = ? AND node_name = ?", clusterID, nodeName).First(&node).Error; err == nil {
				pod.NodeID = &node.ID
				c.db.Save(&pod)
			}
		}
	}
	
	return nil
}

// correlateServiceAccount links ServiceAccount to Pods
func (c *Correlator) correlateServiceAccount(item *normalizer.NormalizedInventoryItem, clusterID string, rawData map[string]interface{}) error {
	// Find service account
	var sa models.ServiceAccount
	if err := c.db.Where("cluster_id = ? AND name = ? AND namespace = ?", 
		clusterID, item.Name, item.Namespace).First(&sa).Error; err != nil {
		return nil // SA not found, will be created by sync
	}
	
	// Find pods using this service account
	var pods []models.Pod
	c.db.Where("cluster_id = ? AND service_account = ? AND namespace = ?",
		clusterID, item.Name, item.Namespace).Find(&pods)
	
	// Update linked_pods
	podUIDs := make([]string, len(pods))
	for i, pod := range pods {
		podUIDs[i] = pod.UID
	}
	
	podUIDsJSON, _ := json.Marshal(podUIDs)
	sa.LinkedPods = string(podUIDsJSON)
	c.db.Save(&sa)
	
	return nil
}

// correlateRole handles Role/ClusterRole correlation
func (c *Correlator) correlateRole(item *normalizer.NormalizedInventoryItem, clusterID string, rawData map[string]interface{}) error {
	// Roles are already stored by sync process
	// Just log for now
	log.Printf("[Correlator] Role correlation: %s/%s", item.Namespace, item.Name)
	return nil
}

// correlateRoleBinding links RoleBinding to Roles and Subjects
func (c *Correlator) correlateRoleBinding(item *normalizer.NormalizedInventoryItem, clusterID string, rawData map[string]interface{}) error {
	// RoleBindings are already stored by sync process
	// Just log for now
	log.Printf("[Correlator] RoleBinding correlation: %s/%s", item.Namespace, item.Name)
	return nil
}

// CorrelateEvent correlates a runtime event with existing resources
func (c *Correlator) CorrelateEvent(event *normalizer.NormalizedEvent) error {
	log.Printf("[Correlator] Correlating event: event_id=%s, pod_uid=%s", 
		event.EventID, event.PodUID)
	
	// Find pod
	var pod models.Pod
	if err := c.db.Where("uid = ?", event.PodUID).First(&pod).Error; err != nil {
		// Pod not found, skip correlation
		return nil
	}
	
	// Find service account
	if pod.ServiceAccount != "" {
		var sa models.ServiceAccount
		if err := c.db.Where("cluster_id = ? AND name = ? AND namespace = ?",
			pod.ClusterID, pod.ServiceAccount, pod.Namespace).First(&sa).Error; err == nil {
			// Update last_used timestamp
			now := time.Now()
			sa.LastUsed = &now
			c.db.Save(&sa)
		}
	}
	
	// TODO: Update risk scores based on event type and severity
	
	return nil
}

