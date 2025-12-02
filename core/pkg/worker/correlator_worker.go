package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
)

// CorrelatorWorker builds relationships between resources
type CorrelatorWorker struct {
	js          nats.JetStreamContext
	db          *gorm.DB
	graphEngine interface{} // AgeGraphEngine interface (optional, for dual-write)
}

// NewCorrelatorWorker creates a new correlator worker
func NewCorrelatorWorker(js nats.JetStreamContext, db *gorm.DB) *CorrelatorWorker {
	return &CorrelatorWorker{
		js:          js,
		db:          db,
		graphEngine: nil, // Will be set if AGE is available
	}
}

// SetGraphEngine sets the graph engine for dual-write
func (w *CorrelatorWorker) SetGraphEngine(graphEngine interface{}) {
	w.graphEngine = graphEngine
}

// Process processes a normalized message
func (w *CorrelatorWorker) Process(ctx context.Context, msg *nats.Msg) error {
	var normalizedData map[string]interface{}
	if err := json.Unmarshal(msg.Data, &normalizedData); err != nil {
		return fmt.Errorf("failed to unmarshal normalized item: %w", err)
	}

	kind, _ := normalizedData["kind"].(string)
	name, _ := normalizedData["name"].(string)
	namespace, _ := normalizedData["namespace"].(string)
	clusterID, _ := normalizedData["cluster_id"].(string)
	if clusterID == "" {
		clusterID = "default"
	}

	log.Printf("[CorrelatorWorker] Processing normalized item: kind=%s, name=%s/%s, cluster=%s",
		kind, namespace, name, clusterID)

	// Store item in database based on kind
	switch kind {
	case "Pod":
		return w.processPod(normalizedData, clusterID)
	case "ServiceAccount":
		return w.processServiceAccount(normalizedData, clusterID)
	case "Role", "ClusterRole":
		return w.processRole(normalizedData, clusterID)
	case "RoleBinding", "ClusterRoleBinding":
		return w.processRoleBinding(normalizedData, clusterID)
	default:
		log.Printf("[CorrelatorWorker] Unknown kind: %s, skipping", kind)
		return nil
	}
}

// processPod stores pod and links to service account
func (w *CorrelatorWorker) processPod(data map[string]interface{}, clusterID string) error {
	uid, _ := data["uid"].(string)
	name, _ := data["name"].(string)
	namespace, _ := data["namespace"].(string)
	rawJSON, _ := data["raw_json"].(string)

	// Parse raw JSON to extract service account
	var k8sPod map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &k8sPod); err != nil {
		return fmt.Errorf("failed to parse pod JSON: %w", err)
	}

	spec, _ := k8sPod["spec"].(map[string]interface{})
	serviceAccountName := "default"
	if sa, ok := spec["serviceAccountName"].(string); ok && sa != "" {
		serviceAccountName = sa
	}

	// Note: Namespace table exists but we'll skip creation for now
	// to keep the implementation simple

	// Extract containers from spec
	var containersJSON string
	if spec, ok := k8sPod["spec"].(map[string]interface{}); ok {
		if containers, ok := spec["containers"].([]interface{}); ok {
			containersBytes, err := json.Marshal(containers)
			if err == nil {
				// Validate JSON is valid before storing
				var testJSON interface{}
				if json.Unmarshal(containersBytes, &testJSON) == nil {
					containersJSON = string(containersBytes)
				} else {
					log.Printf("[CorrelatorWorker] Invalid JSON after marshal, using empty array")
					containersJSON = "[]"
				}
			} else {
				log.Printf("[CorrelatorWorker] Failed to marshal containers: %v", err)
				containersJSON = "[]"
			}
		} else {
			containersJSON = "[]"
		}
	} else {
		containersJSON = "[]"
	}
	
	// Ensure containersJSON is never empty - use empty array if empty string
	if containersJSON == "" {
		containersJSON = "[]"
	}

	// Validate JSON before storing
	var testJSON interface{}
	if err := json.Unmarshal([]byte(containersJSON), &testJSON); err != nil {
		log.Printf("[CorrelatorWorker] Warning: containersJSON is invalid JSON, using empty array: %v", err)
		containersJSON = "[]"
	}

	// Ensure cluster exists first
	cluster := models.Cluster{ID: clusterID, Name: clusterID}
	w.db.FirstOrCreate(&cluster)

	// Use raw SQL with jsonb cast to avoid GORM encoding issues
	// Check if pod exists
	var existingPod models.Pod
	exists := w.db.Where("uid = ?", uid).First(&existingPod).Error == nil

	if exists {
		// Update existing pod using raw SQL for jsonb field
		updateSQL := `UPDATE pods SET 
			cluster_id = ?, 
			name = ?, 
			namespace = ?, 
			service_account = ?, 
			containers = ?::jsonb,
			updated_at = ?
			WHERE uid = ?`
		if err := w.db.Exec(updateSQL, clusterID, name, namespace, serviceAccountName, containersJSON, time.Now(), uid).Error; err != nil {
			return fmt.Errorf("failed to update pod: %w", err)
		}
	} else {
		// Create new pod using raw SQL
		insertSQL := `INSERT INTO pods (uid, cluster_id, name, namespace, service_account, containers, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?::jsonb, ?, ?)`
		if err := w.db.Exec(insertSQL, uid, clusterID, name, namespace, serviceAccountName, containersJSON, time.Now(), time.Now()).Error; err != nil {
			return fmt.Errorf("failed to insert pod: %w", err)
		}
	}

	log.Printf("[CorrelatorWorker] Stored pod: %s/%s, serviceAccount=%s", namespace, name, serviceAccountName)
	return nil
}

// processServiceAccount stores service account
func (w *CorrelatorWorker) processServiceAccount(data map[string]interface{}, clusterID string) error {
	// Ensure cluster exists
	cluster := models.Cluster{ID: clusterID, Name: clusterID}
	w.db.FirstOrCreate(&cluster)

	uid, _ := data["uid"].(string)
	name, _ := data["name"].(string)
	namespace, _ := data["namespace"].(string)

	// Handle labels - ensure valid JSON
	labelsVal := data["labels"]
	if labelsVal == nil {
		labelsVal = map[string]interface{}{}
	}
	labelsJSON, _ := json.Marshal(labelsVal)
	if len(labelsJSON) == 0 || string(labelsJSON) == "null" {
		labelsJSON = []byte("{}")
	}

	// Handle secrets - ensure valid JSON array
	secretsVal := data["secrets"]
	if secretsVal == nil {
		secretsVal = []interface{}{}
	}
	secretsJSON, _ := json.Marshal(secretsVal)
	if len(secretsJSON) == 0 || string(secretsJSON) == "null" {
		secretsJSON = []byte("[]")
	}

	// Handle linked_pods - ensure valid JSON array
	linkedPodsJSON := []byte("[]") // Default to empty array
	now := time.Now()
	sa := models.ServiceAccount{
		ClusterID:  clusterID,
		Name:       name,
		Namespace:  namespace,
		UID:        uid,
		Labels:     string(labelsJSON),
		Secrets:    string(secretsJSON),
		LinkedPods: string(linkedPodsJSON),
		LastUsed:   &now,
	}

	result := w.db.Where("cluster_id = ? AND namespace = ? AND name = ?", clusterID, namespace, name).FirstOrCreate(&sa)
	if result.Error != nil {
		return fmt.Errorf("failed to upsert service account: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		if err := w.db.Model(&sa).Where("cluster_id = ? AND namespace = ? AND name = ?", clusterID, namespace, name).
			Updates(map[string]interface{}{
				"uid":        uid,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("failed to update service account: %w", err)
		}
	}

	log.Printf("[CorrelatorWorker] Stored service account: %s/%s", namespace, name)
	return nil
}

// processRole stores role or cluster role
func (w *CorrelatorWorker) processRole(data map[string]interface{}, clusterID string) error {
	// Ensure cluster exists
	cluster := models.Cluster{ID: clusterID, Name: clusterID}
	w.db.FirstOrCreate(&cluster)

	uid, _ := data["uid"].(string)
	name, _ := data["name"].(string)
	namespace, _ := data["namespace"].(string)
	kind, _ := data["kind"].(string)
	rawJSON, _ := data["raw_json"].(string)

	// Parse raw JSON to extract rules
	var k8sRole struct {
		Rules []interface{} `json:"rules"`
	}
	rulesJSON := []byte("[]") // Default to empty array
	if rawJSON != "" {
		if err := json.Unmarshal([]byte(rawJSON), &k8sRole); err != nil {
			log.Printf("[CorrelatorWorker] Failed to parse role JSON for %s/%s: %v", namespace, name, err)
			// Continue with empty rules
		} else {
			rulesJSON, _ = json.Marshal(k8sRole.Rules)
			if len(rulesJSON) == 0 || string(rulesJSON) == "null" {
				rulesJSON = []byte("[]")
			}
		}
	}

	if kind == "ClusterRole" {
		role := models.ClusterRole{
			ClusterID: clusterID,
			Name:      name,
			UID:       uid,
			Rules:     string(rulesJSON),
		}
		result := w.db.Where("cluster_id = ? AND name = ?", clusterID, name).FirstOrCreate(&role)
		if result.Error != nil {
			return fmt.Errorf("failed to upsert cluster role: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if err := w.db.Model(&role).Where("cluster_id = ? AND name = ?", clusterID, name).
				Updates(map[string]interface{}{"uid": uid, "updated_at": time.Now()}).Error; err != nil {
				return fmt.Errorf("failed to update cluster role: %w", err)
			}
		}
		log.Printf("[CorrelatorWorker] Stored cluster role: %s", name)
	} else {
		role := models.Role{
			ClusterID: clusterID,
			Name:      name,
			Namespace: namespace,
			UID:       uid,
			Rules:     string(rulesJSON),
		}
		result := w.db.Where("cluster_id = ? AND namespace = ? AND name = ?", clusterID, namespace, name).FirstOrCreate(&role)
		if result.Error != nil {
			return fmt.Errorf("failed to upsert role: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if err := w.db.Model(&role).Where("cluster_id = ? AND namespace = ? AND name = ?", clusterID, namespace, name).
				Updates(map[string]interface{}{"uid": uid, "updated_at": time.Now()}).Error; err != nil {
				return fmt.Errorf("failed to update role: %w", err)
			}
		}
		log.Printf("[CorrelatorWorker] Stored role: %s/%s", namespace, name)
	}

	return nil
}

// processRoleBinding stores role binding and links to role
func (w *CorrelatorWorker) processRoleBinding(data map[string]interface{}, clusterID string) error {
	// Ensure cluster exists
	cluster := models.Cluster{ID: clusterID, Name: clusterID}
	w.db.FirstOrCreate(&cluster)

	uid, _ := data["uid"].(string)
	name, _ := data["name"].(string)
	namespace, _ := data["namespace"].(string)
	kind, _ := data["kind"].(string)
	rawJSON, _ := data["raw_json"].(string)

	// Parse to extract role reference and subjects
	var k8sRB map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &k8sRB); err != nil {
		return fmt.Errorf("failed to parse rolebinding JSON: %w", err)
	}

	roleRef, _ := k8sRB["roleRef"].(map[string]interface{})
	roleName := ""
	if roleRef != nil {
		roleName, _ = roleRef["name"].(string)
	}
	subjects, _ := k8sRB["subjects"].([]interface{})
	subjectsJSON, _ := json.Marshal(subjects)
	if len(subjectsJSON) == 0 {
		subjectsJSON = []byte("[]")
	}
	roleRefJSON, _ := json.Marshal(roleRef)
	if len(roleRefJSON) == 0 {
		roleRefJSON = []byte("{}")
	}

	if kind == "ClusterRoleBinding" {
		rb := models.ClusterRoleBinding{
			ClusterID: clusterID,
			Name:      name,
			UID:       uid,
			RoleRef:   string(roleRefJSON),
			Subjects:  string(subjectsJSON),
		}
		result := w.db.Where("cluster_id = ? AND name = ?", clusterID, name).FirstOrCreate(&rb)
		if result.Error != nil {
			return fmt.Errorf("failed to upsert cluster role binding: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if err := w.db.Model(&rb).Where("cluster_id = ? AND name = ?", clusterID, name).
				Updates(map[string]interface{}{
					"uid":        uid,
					"role_ref":   string(roleRefJSON),
					"subjects":   string(subjectsJSON),
					"updated_at": time.Now(),
				}).Error; err != nil {
				return fmt.Errorf("failed to update cluster role binding: %w", err)
			}
		}
		log.Printf("[CorrelatorWorker] Stored cluster role binding: %s -> %s", name, roleName)
	} else {
		rb := models.RoleBinding{
			ClusterID: clusterID,
			Name:      name,
			Namespace: namespace,
			UID:       uid,
			RoleRef:   string(roleRefJSON),
			Subjects:  string(subjectsJSON),
		}
		result := w.db.Where("cluster_id = ? AND namespace = ? AND name = ?", clusterID, namespace, name).FirstOrCreate(&rb)
		if result.Error != nil {
			return fmt.Errorf("failed to upsert role binding: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if err := w.db.Model(&rb).Where("cluster_id = ? AND namespace = ? AND name = ?", clusterID, namespace, name).
				Updates(map[string]interface{}{
					"uid":        uid,
					"role_ref":   string(roleRefJSON),
					"subjects":   string(subjectsJSON),
					"updated_at": time.Now(),
				}).Error; err != nil {
				return fmt.Errorf("failed to update role binding: %w", err)
			}
		}
		log.Printf("[CorrelatorWorker] Stored role binding: %s/%s -> %s", namespace, name, roleName)
	}

	return nil
}

// Subject returns the NATS subject to subscribe to
func (w *CorrelatorWorker) Subject() string {
	return "ksam.normalized.>"
}

// Name returns the worker name
func (w *CorrelatorWorker) Name() string {
	return "correlator"
}
