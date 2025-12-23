package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/fortuna/core/pkg/models"
)

// CorrelatorWorker builds relationships between resources
type CorrelatorWorker struct {
	js          nats.JetStreamContext
	db          *gorm.DB
	graphEngine interface{} // AgeGraphEngine interface (optional, for dual-write)
	k8sClient   kubernetes.Interface
	// Layer 2: Cache validation results (1 min)
	validationCache map[string]time.Time // uid -> validation time
	cacheMutex       sync.RWMutex
}

// NewCorrelatorWorker creates a new correlator worker
func NewCorrelatorWorker(js nats.JetStreamContext, db *gorm.DB) *CorrelatorWorker {
	// Layer 2: Initialize K8s client for validation
	var k8sClient kubernetes.Interface
	config, err := rest.InClusterConfig()
	if err == nil {
		client, err := kubernetes.NewForConfig(config)
		if err == nil {
			k8sClient = client
			log.Printf("[CorrelatorWorker] K8s client initialized for validation")
		} else {
			log.Printf("[CorrelatorWorker] Warning: Failed to create K8s client: %v (validation disabled)", err)
		}
	} else {
		log.Printf("[CorrelatorWorker] Warning: Not running in cluster, K8s validation disabled: %v", err)
	}

	return &CorrelatorWorker{
		js:               js,
		db:               db,
		graphEngine:      nil, // Will be set if AGE is available
		k8sClient:        k8sClient,
		validationCache:  make(map[string]time.Time),
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
	
	// Layer 1: Extract eventType
	eventType, _ := normalizedData["event_type"].(string)
	if eventType == "" {
		eventType = "Added" // Default
	}

	log.Printf("[CorrelatorWorker] Processing normalized item: kind=%s, name=%s/%s, cluster=%s, eventType=%s",
		kind, namespace, name, clusterID, eventType)

	// Store item in database based on kind
	switch kind {
	case "Pod":
		return w.processPod(normalizedData, clusterID, eventType)
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
// Layer 1: Handle DELETE events to prevent ghost pods
func (w *CorrelatorWorker) processPod(data map[string]interface{}, clusterID string, eventType string) error {
	// Layer 1: Handle DELETE event
	if eventType == "Deleted" {
		uid, _ := data["uid"].(string)
		name, _ := data["name"].(string)
		namespace, _ := data["namespace"].(string)
		
		if uid == "" {
			log.Printf("[CorrelatorWorker] DELETE event missing UID, skipping")
			return nil
		}
		
		// Soft delete pod
		result := w.db.Model(&models.Pod{}).Where("uid = ? AND deleted_at IS NULL", uid).Update("deleted_at", time.Now())
		if result.Error != nil {
			return fmt.Errorf("failed to soft delete pod: %w", result.Error)
		}
		
		if result.RowsAffected > 0 {
			log.Printf("[CorrelatorWorker] Soft-deleted pod %s/%s (UID: %s) from DELETE event (Layer 1: Prevention)", 
				namespace, name, uid)
		} else {
			log.Printf("[CorrelatorWorker] Pod %s/%s (UID: %s) not found or already deleted", 
				namespace, name, uid)
		}
		return nil
	}
	
	// Continue with normal processing for Added/Modified events
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
	// Check if pod exists (only active pods, not soft-deleted)
	var existingPod models.Pod
	exists := w.db.Where("uid = ? AND deleted_at IS NULL", uid).First(&existingPod).Error == nil

	if exists {
		// Update existing pod using raw SQL for jsonb field
		updateSQL := `UPDATE pods SET 
			cluster_id = ?, 
			name = ?, 
			namespace = ?, 
			service_account = ?, 
			containers = ?::jsonb,
			updated_at = ?
			WHERE uid = ? AND deleted_at IS NULL`
		if err := w.db.Exec(updateSQL, clusterID, name, namespace, serviceAccountName, containersJSON, time.Now(), uid).Error; err != nil {
			return fmt.Errorf("failed to update pod: %w", err)
		}
	} else {
		// Check if pod was soft-deleted - only restore if it's recent (within last hour)
		// This prevents restoring old deleted pods from NATS stream
		var deletedPod models.Pod
		deletedExists := w.db.Unscoped().Where("uid = ? AND deleted_at IS NOT NULL AND deleted_at > NOW() - INTERVAL '1 hour'", uid).First(&deletedPod).Error == nil
		
		if deletedExists {
			// Layer 2: Validate pod exists in K8s before restoring
			if w.validatePodExists(namespace, name, uid) {
				// Restore recently soft-deleted pod (likely just deleted, now back in K8s)
				restoreSQL := `UPDATE pods SET 
					cluster_id = ?, 
					name = ?, 
					namespace = ?, 
					service_account = ?, 
					containers = ?::jsonb,
					deleted_at = NULL,
					updated_at = ?
					WHERE uid = ?`
				if err := w.db.Exec(restoreSQL, clusterID, name, namespace, serviceAccountName, containersJSON, time.Now(), uid).Error; err != nil {
					return fmt.Errorf("failed to restore pod: %w", err)
				}
				log.Printf("[CorrelatorWorker] Restored recently soft-deleted pod: %s/%s", namespace, name)
			} else {
				log.Printf("[CorrelatorWorker] Skipped restoring pod %s/%s (not found in K8s)", namespace, name)
			}
		} else {
			// Layer 2: Validate pod exists in K8s before creating
			if !w.validatePodExists(namespace, name, uid) {
				log.Printf("[CorrelatorWorker] Skipped creating pod %s/%s (not found in K8s - likely ghost pod from old NATS message)", namespace, name)
				return nil // Skip creating ghost pods
			}
			
			// Create new pod using raw SQL
			insertSQL := `INSERT INTO pods (uid, cluster_id, name, namespace, service_account, containers, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?::jsonb, ?, ?)`
			if err := w.db.Exec(insertSQL, uid, clusterID, name, namespace, serviceAccountName, containersJSON, time.Now(), time.Now()).Error; err != nil {
				return fmt.Errorf("failed to insert pod: %w", err)
			}
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
			// Update existing role - include rules to ensure latest permissions are stored
			if err := w.db.Model(&role).Where("cluster_id = ? AND name = ?", clusterID, name).
				Updates(map[string]interface{}{
					"uid":        uid,
					"rules":      string(rulesJSON),
					"updated_at": time.Now(),
				}).Error; err != nil {
				return fmt.Errorf("failed to update cluster role: %w", err)
			}
			log.Printf("[CorrelatorWorker] Updated cluster role: %s (rules updated)", name)
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
			// Update existing role - include rules to ensure latest permissions are stored
			// This is critical for auto-resolution: if role rules change, database must reflect it
			if err := w.db.Model(&role).Where("cluster_id = ? AND namespace = ? AND name = ?", clusterID, namespace, name).
				Updates(map[string]interface{}{
					"uid":        uid,
					"rules":      string(rulesJSON),
					"updated_at": time.Now(),
				}).Error; err != nil {
				return fmt.Errorf("failed to update role: %w", err)
			}
			log.Printf("[CorrelatorWorker] Updated role: %s/%s (rules updated)", namespace, name)
		} else {
			log.Printf("[CorrelatorWorker] Created new role: %s/%s", namespace, name)
		}
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

// validatePodExists validates if a pod exists in Kubernetes (Layer 2: Real-time Validation)
// Uses cache to avoid excessive K8s API calls (1 min cache)
func (w *CorrelatorWorker) validatePodExists(namespace, name, uid string) bool {
	// If no K8s client, skip validation (for development/testing)
	if w.k8sClient == nil {
		return true // Allow if validation disabled
	}

	// Layer 2: Check cache first (1 min TTL)
	w.cacheMutex.RLock()
	cachedTime, cached := w.validationCache[uid]
	w.cacheMutex.RUnlock()

	if cached && time.Since(cachedTime) < 1*time.Minute {
		// Cache hit - assume valid (we validated recently)
		return true
	}

	// Cache miss or expired - validate with K8s API
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pod, err := w.k8sClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		// Pod not found or error - mark as invalid
		log.Printf("[CorrelatorWorker] Validation failed for pod %s/%s (UID: %s): %v", namespace, name, uid, err)
		
		// Cache negative result (shorter TTL - 30 seconds)
		w.cacheMutex.Lock()
		w.validationCache[uid] = time.Now()
		w.cacheMutex.Unlock()
		
		return false
	}

	// Verify UID matches
	if string(pod.UID) != uid {
		log.Printf("[CorrelatorWorker] Validation failed: UID mismatch for pod %s/%s (expected: %s, got: %s)", 
			namespace, name, uid, pod.UID)
		return false
	}

	// Valid pod - cache result
	w.cacheMutex.Lock()
	w.validationCache[uid] = time.Now()
	w.cacheMutex.Unlock()

	return true
}
