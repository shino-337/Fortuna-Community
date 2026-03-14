package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/pkg/capability"
	"github.com/fortuna/core/pkg/lifecycle"
	"github.com/fortuna/core/pkg/models"
)

const (
	// Audit log TTL - delete logs older than 90 days
	auditLogTTL = 90 * 24 * time.Hour
)

// isTableMissingErr returns true if err indicates a missing table (e.g. SQLite "no such table").
// Used to avoid noisy PCE logs when test DB is torn down before async goroutine runs.
func isTableMissingErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no such table") || strings.Contains(s, "does not exist")
}

func getStalePodCutoff() time.Duration {
	if m := os.Getenv("STALE_POD_CUTOFF_MINUTES"); m != "" {
		if n, err := strconv.Atoi(m); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}
	// Default with buffer for SYNC_INTERVAL=5m to avoid accidental pruning.
	return 30 * time.Minute
}

// AgentService handles data from agents
type AgentService struct {
	db                 *gorm.DB
	logger             *log.Logger
	systemUserID       uint
	systemUserOnce     sync.Once
	podInstanceManager *lifecycle.PodInstanceManager
}

// NewAgentService creates a new agent service
func NewAgentService(db *gorm.DB) *AgentService {
	writer := io.MultiWriter(os.Stdout)
	return &AgentService{
		db:                 db,
		logger:             log.New(writer, "[AgentService] ", log.LstdFlags|log.Lmicroseconds),
		podInstanceManager: lifecycle.NewPodInstanceManager(db),
	}
}

// getSystemUserID retrieves or creates a system user for audit logs
func (s *AgentService) getSystemUserID() uint {
	s.systemUserOnce.Do(func() {
		id, err := s.ensureSystemUser()
		if err != nil {
			s.logger.Printf("Failed to ensure system user: %v", err)
			return
		}
		s.systemUserID = id
	})
	return s.systemUserID
}

// ensureSystemUser ensures a system user exists for audit logging
func (s *AgentService) ensureSystemUser() (uint, error) {
	var user models.User
	// Prefer dedicated system user
	if err := s.db.Where("username = ?", "system").First(&user).Error; err == nil {
		return user.ID, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	// Fallback to first admin if available
	if err := s.db.Where("role = ?", models.RoleAdmin).Order("id ASC").First(&user).Error; err == nil {
		return user.ID, nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	// Create system user automatically
	password, err := generateRandomPassword()
	if err != nil {
		return 0, err
	}
	hashed, err := auth.HashPassword(password)
	if err != nil {
		return 0, err
	}

	user = models.User{
		Username: "system",
		Email:    "system@fortuna.local",
		Password: hashed,
		Role:     models.RoleAdmin,
		Active:   true,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return 0, err
	}
	s.logger.Printf("Created system user for audit logs (username=system)")
	return user.ID, nil
}

func generateRandomPassword() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// extractBoolFlag extracts a boolean flag from the data map, handling multiple types
func extractBoolFlag(data map[string]interface{}, key string) (bool, bool) {
	val, exists := data[key]
	if !exists {
		return false, false
	}

	switch v := val.(type) {
	case bool:
		return v, true
	case float64:
		return v != 0, true
	case string:
		lowerV := ""
		for _, c := range v {
			if c >= 'A' && c <= 'Z' {
				lowerV += string(c + 32)
			} else {
				lowerV += string(c)
			}
		}
		return lowerV == "true" || v == "1", true
	default:
		return false, false
	}
}

// equalTimePtr returns true if both time pointers are nil or point to equal times.
func equalTimePtr(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

// createAuditLog creates an audit log entry directly (no buffering, no throttling)
func (s *AgentService) createAuditLog(clusterID, action, resource, resourceID, namespace, name string) {
	systemUserID := s.getSystemUserID()
	if systemUserID == 0 {
		s.logger.Printf("Cannot create audit log: system user not available")
		return
	}

	auditLog := models.AuditLog{
		ClusterID:  clusterID,
		UserID:     systemUserID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		User:       "system",
		IP:         "agent-sync",
		Details:    `{"source":"agent-sync","namespace":"` + namespace + `","name":"` + name + `"}`,
	}

	if err := s.db.Create(&auditLog).Error; err != nil {
		s.logger.Printf("❌ Failed to create audit log (action=%s resource=%s id=%s): %v", action, resource, resourceID, err)
		return
	}
	s.logger.Printf("✅ Created audit log: action=%s resource=%s id=%s name=%s/%s", action, resource, resourceID, namespace, name)
}

// SyncData syncs collected data from agent. SSOT: cluster_id immutable; only mutable fields updated when cluster exists.
func (s *AgentService) SyncData(clusterID string, clusterName string, source, k8sVersion, distribution string, data map[string]interface{}) error {
	displayName := clusterName
	if displayName == "" {
		displayName = clusterID
	}
	now := time.Now()

	var existing models.Cluster
	err := s.db.First(&existing, "id = ?", clusterID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("lookup cluster: %w", err)
	}

	if err == gorm.ErrRecordNotFound {
		// Create new cluster (cluster_id not in DB)
		cluster := models.Cluster{
			ID:           clusterID,
			Name:         displayName,
			Source:       source,
			K8sVersion:   k8sVersion,
			Distribution: distribution,
			Status:       "active",
			LastSync:     now,
		}
		if err := s.db.Create(&cluster).Error; err != nil {
			s.logger.Printf("❌ Failed to create cluster id=%q: %v", clusterID, err)
			return fmt.Errorf("create cluster: %w", err)
		}
		s.logger.Printf("✅ Cluster created: id=%q name=%q source=%s", clusterID, displayName, source)
	} else {
		// Update only mutable fields (never create new cluster just because name/source changed)
		updates := map[string]interface{}{
			"name":       displayName,
			"status":     "active",
			"last_sync":  now,
			"updated_at": now,
		}
		if source != "" {
			updates["source"] = source
		}
		if k8sVersion != "" {
			updates["k8s_version"] = k8sVersion
		}
		if distribution != "" {
			updates["distribution"] = distribution
		}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			s.logger.Printf("❌ Failed to update cluster id=%q: %v", clusterID, err)
			return fmt.Errorf("update cluster: %w", err)
		}
		s.logger.Printf("✅ Cluster updated: id=%q name=%q (mutable only)", clusterID, displayName)
	}

	// Determine sync type from explicit flags ONLY
	isFullSync, hasFullFlag := extractBoolFlag(data, "isFullSync")
	isDeltaSync, hasDeltaFlag := extractBoolFlag(data, "isDeltaSync")

	// Default: if no flags provided, assume delta sync (safer assumption)
	if !hasFullFlag && !hasDeltaFlag {
		isDeltaSync = true
		isFullSync = false
		s.logger.Printf("⚠️  No sync flags provided, defaulting to isDeltaSync=true")
	} else if hasFullFlag && !hasDeltaFlag {
		isDeltaSync = !isFullSync
		s.logger.Printf("📌 Only isFullSync=%v provided, setting isDeltaSync=%v", isFullSync, isDeltaSync)
	} else if !hasFullFlag && hasDeltaFlag {
		isFullSync = !isDeltaSync
		s.logger.Printf("📌 Only isDeltaSync=%v provided, setting isFullSync=%v", isDeltaSync, isFullSync)
	} else {
		s.logger.Printf("📌 Both flags provided - isFullSync=%v, isDeltaSync=%v", isFullSync, isDeltaSync)
	}

	// Process ServiceAccounts
	if err := s.processSyncedServiceAccounts(clusterID, data, isFullSync, isDeltaSync); err != nil {
		return err
	}

	// Process RBAC resources (only during full sync)
	if isFullSync {
		if err := s.processSyncedRoles(clusterID, data); err != nil {
			s.logger.Printf("❌ Error processing roles: %v", err)
		}
		if err := s.processSyncedClusterRoles(clusterID, data); err != nil {
			s.logger.Printf("❌ Error processing cluster roles: %v", err)
		}
		if err := s.processSyncedRoleBindings(clusterID, data); err != nil {
			s.logger.Printf("❌ Error processing role bindings: %v", err)
		}
		if err := s.processSyncedClusterRoleBindings(clusterID, data); err != nil {
			s.logger.Printf("❌ Error processing cluster role bindings: %v", err)
		}
		if err := s.processSyncedPods(clusterID, data, isFullSync); err != nil {
			s.logger.Printf("❌ Error processing pods: %v", err)
		}
		// Fallback reconciliation for missed delete events.
		go s.cleanupStalePods(clusterID)
		if err := s.processSyncedDeployments(clusterID, data); err != nil {
			s.logger.Printf("❌ Error processing deployments: %v", err)
		}
		if err := s.processSyncedReplicaSets(clusterID, data); err != nil {
			s.logger.Printf("❌ Error processing replicasets: %v", err)
		}
	} else {
		// For delta sync, still process pods if they are in the payload
		// This allows real-time pod updates
		if pods, ok := data["pods"].([]interface{}); ok && len(pods) > 0 {
			if err := s.processSyncedPods(clusterID, data, false); err != nil {
				s.logger.Printf("❌ Error processing pods (delta): %v", err)
			}
		}
	}

	// Cleanup old audit logs (TTL)
	go s.cleanupOldAuditLogs()

	// Cleanup stale clusters: soft-delete clusters not synced in 90 days so dashboard/DB don't keep old env data
	go s.cleanupStaleClusters()

	return nil
}

// processSyncedServiceAccounts handles ServiceAccount sync.
// When payload is missing or empty we do NOT delete existing SAs (transient collection failure could wipe data).
func (s *AgentService) processSyncedServiceAccounts(clusterID string, data map[string]interface{}, isFullSync, isDeltaSync bool) error {
	sas, ok := data["serviceAccounts"].([]interface{})
	if !ok || len(sas) == 0 {
		s.logger.Printf("WARN full sync missing key 'serviceAccounts' for cluster %q, skipping delete (transient collection failure?)", clusterID)
		return nil
	}

	s.logger.Printf("📦 Processing %d serviceAccounts (isFullSync=%v, isDeltaSync=%v)", len(sas), isFullSync, isDeltaSync)

	syncedUIDs := make(map[string]bool)

	for _, saData := range sas {
		saMap, ok := saData.(map[string]interface{})
		if !ok {
			continue
		}

		name, nameOK := saMap["name"].(string)
		namespace, nsOK := saMap["namespace"].(string)
		uid, uidOK := saMap["uid"].(string)
		if !nameOK || !nsOK || !uidOK || uid == "" {
			continue
		}

		syncedUIDs[uid] = true

		// Parse labels
		labels := make(map[string]string)
		if labelsData, ok := saMap["labels"].(map[string]interface{}); ok {
			for k, v := range labelsData {
				if strVal, ok := v.(string); ok {
					labels[k] = strVal
				}
			}
		}

		// Parse secrets
		var secrets []string
		if secretsData, ok := saMap["secrets"].([]interface{}); ok {
			for _, sec := range secretsData {
				if secStr, ok := sec.(string); ok {
					secrets = append(secrets, secStr)
				}
			}
		}

		// Parse linked pods
		var linkedPods []string
		if linkedData, ok := saMap["linkedPods"].([]interface{}); ok {
			for _, p := range linkedData {
				if podUID, ok := p.(string); ok {
					linkedPods = append(linkedPods, podUID)
				}
			}
		}

		labelsJSON, _ := json.Marshal(labels)
		secretsJSON, _ := json.Marshal(secrets)
		linkedPodsJSON, _ := json.Marshal(linkedPods)

		sa := models.ServiceAccount{
			ClusterID:  clusterID,
			UID:        uid,
			Name:       name,
			Namespace:  namespace,
			Labels:     string(labelsJSON),
			Secrets:    string(secretsJSON),
			LinkedPods: string(linkedPodsJSON),
		}

		// Check if SA exists in DB
		var existingSA models.ServiceAccount
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existingSA).Error

		if err == nil {
			// SA exists - check if it changed
			changed := existingSA.Name != sa.Name ||
				existingSA.Namespace != sa.Namespace ||
				existingSA.Labels != sa.Labels ||
				existingSA.Secrets != sa.Secrets ||
				existingSA.LinkedPods != sa.LinkedPods

			if changed {
				// Update SA
				s.db.Model(&existingSA).Updates(map[string]interface{}{
					"name":        sa.Name,
					"namespace":   sa.Namespace,
					"labels":      sa.Labels,
					"secrets":     sa.Secrets,
					"linked_pods": sa.LinkedPods,
				})

				resourceID := strconv.Itoa(int(existingSA.ID))

				// For delta sync, always create update audit log
				if isDeltaSync {
					s.createAuditLog(clusterID, "update", "serviceaccount", resourceID, sa.Namespace, sa.Name)
					s.logger.Printf("🔄 Updated SA %s/%s (ID=%d)", sa.Namespace, sa.Name, existingSA.ID)
				}
			} else {
				s.logger.Printf("⏭️  SA %s/%s unchanged, skipping", sa.Namespace, sa.Name)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// SA doesn't exist - create new
			// Check if it was soft-deleted
			var deletedSA models.ServiceAccount
			errDeleted := s.db.Unscoped().Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&deletedSA).Error

			if errDeleted == nil && deletedSA.DeletedAt.Valid {
				// Restore soft-deleted SA
				deletedSA.Name = sa.Name
				deletedSA.Namespace = sa.Namespace
				deletedSA.Labels = sa.Labels
				deletedSA.Secrets = sa.Secrets
				deletedSA.LinkedPods = sa.LinkedPods
				deletedSA.DeletedAt = gorm.DeletedAt{}
				s.db.Save(&deletedSA)

				resourceID := strconv.Itoa(int(deletedSA.ID))
				s.createAuditLog(clusterID, "create", "serviceaccount", resourceID, sa.Namespace, sa.Name)
				s.logger.Printf("🔄 Restored SA %s/%s (ID=%d)", sa.Namespace, sa.Name, deletedSA.ID)
			} else {
				// Create brand new SA
				if err := s.db.Create(&sa).Error; err != nil {
					s.logger.Printf("❌ Failed to create SA %s/%s: %v", sa.Namespace, sa.Name, err)
					continue
				}

				resourceID := strconv.Itoa(int(sa.ID))
				s.createAuditLog(clusterID, "create", "serviceaccount", resourceID, sa.Namespace, sa.Name)
				s.logger.Printf("✨ Created SA %s/%s (ID=%d)", sa.Namespace, sa.Name, sa.ID)
			}
		} else {
			// Database error
			s.logger.Printf("❌ DB error checking SA %s/%s: %v", sa.Namespace, sa.Name, err)
			return err
		}
	}

	// For full sync, delete SAs that are not in the synced list
	if isFullSync && len(syncedUIDs) > 0 {
		keepUIDs := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			keepUIDs = append(keepUIDs, uid)
		}

		var toDelete []models.ServiceAccount
		s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, keepUIDs).Find(&toDelete)

		for _, sa := range toDelete {
			resourceID := strconv.Itoa(int(sa.ID))
			s.createAuditLog(clusterID, "delete", "serviceaccount", resourceID, sa.Namespace, sa.Name)
			s.db.Delete(&sa)
			s.logger.Printf("🗑️  Deleted SA %s/%s (ID=%d) - not in full sync", sa.Namespace, sa.Name, sa.ID)
		}
	}

	return nil
}

// processSyncedRoles handles Role sync (full sync only).
// When payload is missing or empty we do NOT delete existing roles (transient collection failure could wipe data).
func (s *AgentService) processSyncedRoles(clusterID string, data map[string]interface{}) error {
	roles, ok := data["roles"].([]interface{})
	if !ok || len(roles) == 0 {
		s.logger.Printf("WARN full sync missing key 'roles' for cluster %q, skipping delete (transient collection failure?)", clusterID)
		return nil
	}

	s.logger.Printf("📦 Processing %d roles", len(roles))

	syncedUIDs := make(map[string]bool)

	for _, roleData := range roles {
		roleMap, ok := roleData.(map[string]interface{})
		if !ok {
			continue
		}

		name, nameOK := roleMap["name"].(string)
		namespace, nsOK := roleMap["namespace"].(string)
		uid, uidOK := roleMap["uid"].(string)
		if !nameOK || !nsOK || !uidOK || uid == "" {
			continue
		}

		syncedUIDs[uid] = true

		// Parse rules
		var rulesJSON string = "[]"
		if rules, ok := roleMap["rules"].([]interface{}); ok {
			if rulesBytes, err := json.Marshal(rules); err == nil {
				rulesJSON = string(rulesBytes)
			}
		}

		role := models.Role{
			ClusterID: clusterID,
			UID:       uid,
			Name:      name,
			Namespace: namespace,
			Rules:     rulesJSON,
		}

		// Upsert role - check by UID first, then by name+namespace if UID not found
		// This handles cases where role was deleted and recreated with new UID
		var existing models.Role
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existing).Error
		if err == nil {
			// Check if changed
			changed := existing.Name != role.Name ||
				existing.Namespace != role.Namespace ||
				existing.Rules != role.Rules

			if changed {
				// Update existing
				s.db.Model(&existing).Updates(map[string]interface{}{
					"name":       role.Name,
					"namespace":  role.Namespace,
					"rules":      role.Rules,
					"updated_at": time.Now(),
				})
				resourceID := strconv.Itoa(int(existing.ID))
				s.createAuditLog(clusterID, "update", "role", resourceID, namespace, name)
				s.logger.Printf("🔄 Updated Role %s/%s (ID=%d)", namespace, name, existing.ID)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// UID not found - check if role with same name+namespace exists (role was recreated)
			var existingByName models.Role
			errByName := s.db.Where("cluster_id = ? AND name = ? AND namespace = ? AND deleted_at IS NULL", clusterID, name, namespace).
				Order("updated_at DESC").First(&existingByName).Error
			if errByName == nil {
				// Role with same name exists - update it with new UID and rules
				s.db.Model(&existingByName).Updates(map[string]interface{}{
					"uid":        role.UID,
					"rules":      role.Rules,
					"updated_at": time.Now(),
				})
				resourceID := strconv.Itoa(int(existingByName.ID))
				s.createAuditLog(clusterID, "update", "role", resourceID, namespace, name)
				s.logger.Printf("🔄 Updated Role %s/%s (ID=%d) with new UID (role recreated)", namespace, name, existingByName.ID)
				continue // Skip creation
			}
			// Create new
			if err := s.db.Create(&role).Error; err != nil {
				s.logger.Printf("❌ Failed to create Role %s/%s: %v", namespace, name, err)
				continue
			}
			resourceID := strconv.Itoa(int(role.ID))
			s.createAuditLog(clusterID, "create", "role", resourceID, namespace, name)
			s.logger.Printf("✨ Created Role %s/%s (ID=%d)", namespace, name, role.ID)
		}
	}

	// Delete roles not in sync
	if len(syncedUIDs) > 0 {
		keepUIDs := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			keepUIDs = append(keepUIDs, uid)
		}

		var toDelete []models.Role
		s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, keepUIDs).Find(&toDelete)
		for _, role := range toDelete {
			resourceID := strconv.Itoa(int(role.ID))
			s.createAuditLog(clusterID, "delete", "role", resourceID, role.Namespace, role.Name)
			s.db.Delete(&role)
			s.logger.Printf("🗑️  Deleted Role %s/%s (ID=%d) - not in full sync", role.Namespace, role.Name, role.ID)
		}
	}

	return nil
}

// processSyncedClusterRoles handles ClusterRole sync (full sync only).
// When payload is missing or empty we do NOT delete existing cluster roles (transient collection failure could wipe data).
func (s *AgentService) processSyncedClusterRoles(clusterID string, data map[string]interface{}) error {
	clusterRoles, ok := data["clusterRoles"].([]interface{})
	if !ok || len(clusterRoles) == 0 {
		s.logger.Printf("WARN full sync missing key 'clusterRoles' for cluster %q, skipping delete (transient collection failure?)", clusterID)
		return nil
	}

	s.logger.Printf("📦 Processing %d cluster roles", len(clusterRoles))

	syncedUIDs := make(map[string]bool)

	for _, crData := range clusterRoles {
		crMap, ok := crData.(map[string]interface{})
		if !ok {
			continue
		}

		name, nameOK := crMap["name"].(string)
		uid, uidOK := crMap["uid"].(string)
		if !nameOK || !uidOK || uid == "" {
			continue
		}

		syncedUIDs[uid] = true

		// Parse rules
		var rulesJSON string = "[]"
		if rules, ok := crMap["rules"].([]interface{}); ok {
			if rulesBytes, err := json.Marshal(rules); err == nil {
				rulesJSON = string(rulesBytes)
			}
		}

		clusterRole := models.ClusterRole{
			ClusterID: clusterID,
			UID:       uid,
			Name:      name,
			Rules:     rulesJSON,
		}

		// Upsert cluster role
		var existing models.ClusterRole
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existing).Error
		if err == nil {
			// Check if changed
			changed := existing.Name != clusterRole.Name ||
				existing.Rules != clusterRole.Rules

			if changed {
				// Update existing
				s.db.Model(&existing).Updates(map[string]interface{}{
					"name":  clusterRole.Name,
					"rules": clusterRole.Rules,
				})
				resourceID := strconv.Itoa(int(existing.ID))
				s.createAuditLog(clusterID, "update", "clusterrole", resourceID, "", name)
				s.logger.Printf("🔄 Updated ClusterRole %s (ID=%d)", name, existing.ID)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new
			if err := s.db.Create(&clusterRole).Error; err != nil {
				s.logger.Printf("❌ Failed to create ClusterRole %s: %v", name, err)
				continue
			}
			resourceID := strconv.Itoa(int(clusterRole.ID))
			s.createAuditLog(clusterID, "create", "clusterrole", resourceID, "", name)
			s.logger.Printf("✨ Created ClusterRole %s (ID=%d)", name, clusterRole.ID)
		}
	}

	// Delete cluster roles not in sync
	if len(syncedUIDs) > 0 {
		keepUIDs := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			keepUIDs = append(keepUIDs, uid)
		}

		var toDelete []models.ClusterRole
		s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, keepUIDs).Find(&toDelete)
		for _, cr := range toDelete {
			resourceID := strconv.Itoa(int(cr.ID))
			s.createAuditLog(clusterID, "delete", "clusterrole", resourceID, "", cr.Name)
			s.db.Delete(&cr)
			s.logger.Printf("🗑️  Deleted ClusterRole %s (ID=%d) - not in full sync", cr.Name, cr.ID)
		}
	}

	return nil
}

// processSyncedRoleBindings handles RoleBinding sync (full sync only).
// When payload is missing or empty we do NOT delete existing role bindings (transient collection failure could wipe data).
func (s *AgentService) processSyncedRoleBindings(clusterID string, data map[string]interface{}) error {
	roleBindings, ok := data["roleBindings"].([]interface{})
	if !ok || len(roleBindings) == 0 {
		s.logger.Printf("WARN full sync missing key 'roleBindings' for cluster %q, skipping delete (transient collection failure?)", clusterID)
		return nil
	}

	s.logger.Printf("📦 Processing %d role bindings", len(roleBindings))

	syncedUIDs := make(map[string]bool)

	for _, rbData := range roleBindings {
		rbMap, ok := rbData.(map[string]interface{})
		if !ok {
			continue
		}

		name, nameOK := rbMap["name"].(string)
		namespace, nsOK := rbMap["namespace"].(string)
		uid, uidOK := rbMap["uid"].(string)
		if !nameOK || !nsOK || !uidOK || uid == "" {
			continue
		}

		syncedUIDs[uid] = true

		// Parse roleRef
		var roleRefJSON string = "{}"
		if roleRef, ok := rbMap["roleRef"].(map[string]interface{}); ok {
			if refBytes, err := json.Marshal(roleRef); err == nil {
				roleRefJSON = string(refBytes)
			}
		}

		// Parse subjects
		var subjectsJSON string = "[]"
		if subjects, ok := rbMap["subjects"].([]interface{}); ok {
			if subjBytes, err := json.Marshal(subjects); err == nil {
				subjectsJSON = string(subjBytes)
			}
		}

		roleBinding := models.RoleBinding{
			ClusterID: clusterID,
			UID:       uid,
			Name:      name,
			Namespace: namespace,
			RoleRef:   roleRefJSON,
			Subjects:  subjectsJSON,
		}

		// Upsert role binding
		var existing models.RoleBinding
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existing).Error
		if err == nil {
			// Check if changed
			changed := existing.Name != roleBinding.Name ||
				existing.Namespace != roleBinding.Namespace ||
				existing.RoleRef != roleBinding.RoleRef ||
				existing.Subjects != roleBinding.Subjects

			if changed {
				// Update existing
				s.db.Model(&existing).Updates(map[string]interface{}{
					"name":      roleBinding.Name,
					"namespace": roleBinding.Namespace,
					"role_ref":  roleBinding.RoleRef,
					"subjects":  roleBinding.Subjects,
				})
				resourceID := strconv.Itoa(int(existing.ID))
				s.createAuditLog(clusterID, "update", "rolebinding", resourceID, namespace, name)
				s.logger.Printf("🔄 Updated RoleBinding %s/%s (ID=%d)", namespace, name, existing.ID)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new
			if err := s.db.Create(&roleBinding).Error; err != nil {
				s.logger.Printf("❌ Failed to create RoleBinding %s/%s: %v", namespace, name, err)
				continue
			}
			resourceID := strconv.Itoa(int(roleBinding.ID))
			s.createAuditLog(clusterID, "create", "rolebinding", resourceID, namespace, name)
			s.logger.Printf("✨ Created RoleBinding %s/%s (ID=%d)", namespace, name, roleBinding.ID)
		}
	}

	// Delete role bindings not in sync
	if len(syncedUIDs) > 0 {
		keepUIDs := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			keepUIDs = append(keepUIDs, uid)
		}

		var toDelete []models.RoleBinding
		s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, keepUIDs).Find(&toDelete)
		for _, rb := range toDelete {
			resourceID := strconv.Itoa(int(rb.ID))
			s.createAuditLog(clusterID, "delete", "rolebinding", resourceID, rb.Namespace, rb.Name)
			s.db.Delete(&rb)
			s.logger.Printf("🗑️  Deleted RoleBinding %s/%s (ID=%d) - not in full sync", rb.Namespace, rb.Name, rb.ID)
		}
	}

	return nil
}

// processSyncedClusterRoleBindings handles ClusterRoleBinding sync (full sync only).
// When payload is missing or empty we do NOT delete existing cluster role bindings (transient collection failure could wipe data).
func (s *AgentService) processSyncedClusterRoleBindings(clusterID string, data map[string]interface{}) error {
	clusterRoleBindings, ok := data["clusterRoleBindings"].([]interface{})
	if !ok || len(clusterRoleBindings) == 0 {
		s.logger.Printf("WARN full sync missing key 'clusterRoleBindings' for cluster %q, skipping delete (transient collection failure?)", clusterID)
		return nil
	}

	s.logger.Printf("📦 Processing %d cluster role bindings", len(clusterRoleBindings))

	syncedUIDs := make(map[string]bool)

	for _, crbData := range clusterRoleBindings {
		crbMap, ok := crbData.(map[string]interface{})
		if !ok {
			continue
		}

		name, nameOK := crbMap["name"].(string)
		uid, uidOK := crbMap["uid"].(string)
		if !nameOK || !uidOK || uid == "" {
			continue
		}

		syncedUIDs[uid] = true

		// Parse roleRef
		var roleRefJSON string = "{}"
		if roleRef, ok := crbMap["roleRef"].(map[string]interface{}); ok {
			if refBytes, err := json.Marshal(roleRef); err == nil {
				roleRefJSON = string(refBytes)
			}
		}

		// Parse subjects
		var subjectsJSON string = "[]"
		if subjects, ok := crbMap["subjects"].([]interface{}); ok {
			if subjBytes, err := json.Marshal(subjects); err == nil {
				subjectsJSON = string(subjBytes)
			}
		}

		clusterRoleBinding := models.ClusterRoleBinding{
			ClusterID: clusterID,
			UID:       uid,
			Name:      name,
			RoleRef:   roleRefJSON,
			Subjects:  subjectsJSON,
		}

		// Upsert cluster role binding
		var existing models.ClusterRoleBinding
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existing).Error
		if err == nil {
			// Check if changed
			changed := existing.Name != clusterRoleBinding.Name ||
				existing.RoleRef != clusterRoleBinding.RoleRef ||
				existing.Subjects != clusterRoleBinding.Subjects

			if changed {
				// Update existing
				s.db.Model(&existing).Updates(map[string]interface{}{
					"name":     clusterRoleBinding.Name,
					"role_ref": clusterRoleBinding.RoleRef,
					"subjects": clusterRoleBinding.Subjects,
				})
				resourceID := strconv.Itoa(int(existing.ID))
				s.createAuditLog(clusterID, "update", "clusterrolebinding", resourceID, "", name)
				s.logger.Printf("🔄 Updated ClusterRoleBinding %s (ID=%d)", name, existing.ID)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new
			if err := s.db.Create(&clusterRoleBinding).Error; err != nil {
				s.logger.Printf("❌ Failed to create ClusterRoleBinding %s: %v", name, err)
				continue
			}
			resourceID := strconv.Itoa(int(clusterRoleBinding.ID))
			s.createAuditLog(clusterID, "create", "clusterrolebinding", resourceID, "", name)
			s.logger.Printf("✨ Created ClusterRoleBinding %s (ID=%d)", name, clusterRoleBinding.ID)
		}
	}

	// Delete cluster role bindings not in sync
	if len(syncedUIDs) > 0 {
		keepUIDs := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			keepUIDs = append(keepUIDs, uid)
		}

		var toDelete []models.ClusterRoleBinding
		s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, keepUIDs).Find(&toDelete)
		for _, crb := range toDelete {
			resourceID := strconv.Itoa(int(crb.ID))
			s.createAuditLog(clusterID, "delete", "clusterrolebinding", resourceID, "", crb.Name)
			s.db.Delete(&crb)
			s.logger.Printf("🗑️  Deleted ClusterRoleBinding %s (ID=%d) - not in full sync", crb.Name, crb.ID)
		}
	}

	return nil
}

// processSyncedPods handles Pod sync (full sync only).
// When payload is missing or empty we do NOT delete existing pods (transient collection failure could wipe data).
func (s *AgentService) processSyncedPods(clusterID string, data map[string]interface{}, isFullSync bool) error {
	pods, ok := data["pods"].([]interface{})
	if !ok || len(pods) == 0 {
		s.logger.Printf("WARN full sync missing key 'pods' for cluster %q, skipping delete (transient collection failure?)", clusterID)
		return nil
	}

	s.logger.Printf("📦 Processing %d pods", len(pods))

	syncedUIDs := make(map[string]bool)

	for _, podData := range pods {
		podMap, ok := podData.(map[string]interface{})
		if !ok {
			continue
		}

		name, nameOK := podMap["name"].(string)
		namespace, nsOK := podMap["namespace"].(string)
		uid, uidOK := podMap["uid"].(string)
		if !nameOK || !nsOK || !uidOK || uid == "" {
			continue
		}

		syncedUIDs[uid] = true

		serviceAccount := ""
		if sa, ok := podMap["serviceAccountName"].(string); ok {
			serviceAccount = sa
		}

		phase, _ := podMap["phase"].(string)
		if phase != "" && name != "" {
			s.logger.Printf("📦 Pod phase from agent: %s/%s phase=%s", namespace, name, phase)
		}
		nodeName, _ := podMap["nodeName"].(string)
		hostNetwork, _ := podMap["hostNetwork"].(bool)
		hostPID, _ := podMap["hostPID"].(bool)
		hostIPC, _ := podMap["hostIPC"].(bool)
		automountPtr := (*bool)(nil)
		if v, ok := podMap["automountServiceAccountToken"].(bool); ok {
			automountPtr = &v
		}

		containersJSON := "[]"
		volumeMountsJSON := "[]"
		containerSecurityJSON := "{}"
		if containers, ok := podMap["containers"].([]interface{}); ok {
			if b, err := json.Marshal(containers); err == nil {
				containersJSON = string(b)
			}

			volumeMounts := make([]interface{}, 0)
			securityContexts := make(map[string]interface{})
			for _, c := range containers {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := cm["name"].(string)
				if sc, ok := cm["securityContext"]; ok && name != "" {
					securityContexts[name] = sc
				}
				if vms, ok := cm["volumeMounts"].([]interface{}); ok {
					for _, vm := range vms {
						volumeMounts = append(volumeMounts, vm)
					}
				}
			}
			if b, err := json.Marshal(volumeMounts); err == nil {
				volumeMountsJSON = string(b)
			}
			if b, err := json.Marshal(securityContexts); err == nil {
				containerSecurityJSON = string(b)
			}
		}

		volumesJSON := "[]"
		if volumes, ok := podMap["volumes"].([]interface{}); ok {
			if b, err := json.Marshal(volumes); err == nil {
				volumesJSON = string(b)
			}
		}

		tolerationsJSON := "[]"
		if tolerations, ok := podMap["tolerations"].([]interface{}); ok {
			if b, err := json.Marshal(tolerations); err == nil {
				tolerationsJSON = string(b)
			}
		}

		affinityJSON := "{}"
		if affinity, ok := podMap["affinity"]; ok && affinity != nil {
			if b, err := json.Marshal(affinity); err == nil {
				affinityJSON = string(b)
			}
		}

		podSecurityContextJSON := "{}"
		if psc, ok := podMap["podSecurityContext"]; ok && psc != nil {
			if b, err := json.Marshal(psc); err == nil {
				podSecurityContextJSON = string(b)
			}
		}

		// Pod Detail (POD_DETAIL_SPEC): header and overview (accept camelCase from agent or snake_case)
		podIP, _ := podMap["podIP"].(string)
		if podIP == "" {
			podIP, _ = podMap["pod_ip"].(string)
		}
		var startTimeVal models.NullTime
		if st, ok := podMap["startTime"].(string); ok && st != "" {
			if t, err := time.Parse(time.RFC3339, st); err == nil {
				startTimeVal = models.NullTime{Time: &t}
			}
		}
		if startTimeVal.Time == nil {
			if st, ok := podMap["start_time"].(string); ok && st != "" {
				if t, err := time.Parse(time.RFC3339, st); err == nil {
					startTimeVal = models.NullTime{Time: &t}
				}
			}
		}
		restartCount := 0
		if rc, ok := podMap["restartCount"].(float64); ok {
			restartCount = int(rc)
		}
		ownerKind, _ := podMap["ownerKind"].(string)
		ownerName, _ := podMap["ownerName"].(string)
		replicaSetName, _ := podMap["replicaSetName"].(string)
		qosClass, _ := podMap["qosClass"].(string)
		specHash, _ := podMap["specHash"].(string)

		pod := models.Pod{
			ClusterID:                    clusterID,
			UID:                          uid,
			Name:                         name,
			Namespace:                    namespace,
			Phase:                        phase,
			ServiceAccount:               serviceAccount,
			Containers:                   containersJSON,
			ImageDigests:                 "[]",
			PodSecurityContext:           podSecurityContextJSON,
			ContainerSecurityContexts:    containerSecurityJSON,
			VolumeMounts:                 volumeMountsJSON,
			Volumes:                      volumesJSON,
			Tolerations:                  tolerationsJSON,
			Affinity:                     affinityJSON,
			HostNetwork:                  hostNetwork,
			HostPID:                      hostPID,
			HostIPC:                      hostIPC,
			AutomountServiceAccountToken: automountPtr,
			NodeName:                     nodeName,
			PodIP:                        podIP,
			StartTime:                    startTimeVal,
			RestartCount:                 restartCount,
			OwnerKind:                    ownerKind,
			OwnerName:                    ownerName,
			ReplicaSetName:               replicaSetName,
			QoSClass:                     qosClass,
			SpecHash:                     specHash,
		}

		// Upsert pod - use UID as unique identifier
		var existing models.Pod
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existing).Error
		if err == nil {
			// Update existing pod (avoid duplicates)
			changed := existing.Name != pod.Name ||
				existing.Namespace != pod.Namespace ||
				existing.Phase != pod.Phase ||
				existing.ServiceAccount != pod.ServiceAccount ||
				existing.Containers != pod.Containers ||
				existing.PodSecurityContext != pod.PodSecurityContext ||
				existing.ContainerSecurityContexts != pod.ContainerSecurityContexts ||
				existing.VolumeMounts != pod.VolumeMounts ||
				existing.Volumes != pod.Volumes ||
				existing.Tolerations != pod.Tolerations ||
				existing.Affinity != pod.Affinity ||
				existing.HostNetwork != pod.HostNetwork ||
				existing.HostPID != pod.HostPID ||
				existing.HostIPC != pod.HostIPC ||
				existing.NodeName != pod.NodeName ||
				existing.PodIP != pod.PodIP ||
				existing.RestartCount != pod.RestartCount ||
				existing.OwnerKind != pod.OwnerKind ||
				existing.OwnerName != pod.OwnerName ||
				existing.ReplicaSetName != pod.ReplicaSetName ||
				existing.QoSClass != pod.QoSClass ||
				existing.SpecHash != pod.SpecHash ||
				!equalTimePtr(existing.StartTime.Time, pod.StartTime.Time)
			// Trigger PCE when: no hash yet (old agent) or hash changed (backward compat: empty != empty is false, so use explicit existing == "" or different)
			specHashChanged := existing.SpecHash == "" || existing.SpecHash != pod.SpecHash

			if changed {
				// Status-like fields (phase, pod_ip, start_time, restart_count) are handled in the
				// dedicated \"always refresh\" block below so that we never wipe existing values
				// when the payload omits them. Here we only update spec/identity fields.
				upd := map[string]interface{}{
					"name":                            pod.Name,
					"namespace":                       pod.Namespace,
					"service_account":                 pod.ServiceAccount,
					"containers":                      pod.Containers,
					"pod_security_context":            pod.PodSecurityContext,
					"container_security_contexts":     pod.ContainerSecurityContexts,
					"volume_mounts":                   pod.VolumeMounts,
					"volumes":                         pod.Volumes,
					"tolerations":                     pod.Tolerations,
					"affinity":                        pod.Affinity,
					"host_network":                    pod.HostNetwork,
					"host_pid":                        pod.HostPID,
					"host_ipc":                        pod.HostIPC,
					"automount_service_account_token": pod.AutomountServiceAccountToken,
					"node_name":                       pod.NodeName,
					"owner_kind":                      pod.OwnerKind,
					"owner_name":                      pod.OwnerName,
					"replica_set_name":                pod.ReplicaSetName,
					"qos_class":                       pod.QoSClass,
					"spec_hash":                       pod.SpecHash,
				}
				s.db.Model(&existing).Updates(upd)
				s.logger.Printf("🔄 Updated Pod %s/%s (SA: %s)", namespace, name, serviceAccount)
				// Ensure pod instance is active
				ctx := context.Background()
				if err := s.podInstanceManager.EnsureActiveInstance(ctx, uid, namespace, name); err != nil {
					s.logger.Printf("⚠️  Failed to ensure pod instance: %v", err)
				}
				// PCE only when spec_hash changed or agent didn't send hash (existing.SpecHash == ""); run async to not block sync
				if specHashChanged {
					go s.evaluatePodCapabilities(clusterID, uid, pod.SpecHash)
				}
			} else if isFullSync {
				// Touch unchanged pods so stale cleanup can rely on updated_at as last-seen.
				s.db.Model(&existing).Update("updated_at", time.Now())
			}

			// Always refresh status fields (pod_ip, start_time, phase, restart_count) from payload when present,
			// so pods created by older agents or with previously empty status get backfilled on next sync.
			statusUpd := make(map[string]interface{})
			if pod.PodIP != "" {
				statusUpd["pod_ip"] = pod.PodIP
			}
			if pod.StartTime.Time != nil {
				// Pass *time.Time so driver gets a concrete type; NullTime in map may not be handled by GORM
				statusUpd["start_time"] = *pod.StartTime.Time
			}
			if pod.Phase != "" {
				statusUpd["phase"] = pod.Phase
			}
			statusUpd["restart_count"] = pod.RestartCount
			if len(statusUpd) > 0 {
				if err := s.db.Model(&existing).Updates(statusUpd).Error; err != nil {
					s.logger.Printf("⚠️  Failed to refresh pod status (pod_ip/start_time): %v", err)
				} else if _, hasIP := statusUpd["pod_ip"]; hasIP && pod.PodIP != "" {
					s.logger.Printf("📡 Backfilled pod_ip=%s for %s/%s (uid=%s)", pod.PodIP, namespace, name, uid)
				}
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Check if soft-deleted pod exists
			var deletedPod models.Pod
			errDeleted := s.db.Unscoped().Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&deletedPod).Error

			if errDeleted == nil && deletedPod.DeletedAt.Valid {
				// Restore soft-deleted pod
				deletedPod.Name = pod.Name
				deletedPod.Namespace = pod.Namespace
				deletedPod.Phase = pod.Phase
				deletedPod.ServiceAccount = pod.ServiceAccount
				deletedPod.Containers = pod.Containers
				deletedPod.PodSecurityContext = pod.PodSecurityContext
				deletedPod.ContainerSecurityContexts = pod.ContainerSecurityContexts
				deletedPod.VolumeMounts = pod.VolumeMounts
				deletedPod.Volumes = pod.Volumes
				deletedPod.Tolerations = pod.Tolerations
				deletedPod.Affinity = pod.Affinity
				deletedPod.HostNetwork = pod.HostNetwork
				deletedPod.HostPID = pod.HostPID
				deletedPod.HostIPC = pod.HostIPC
				deletedPod.AutomountServiceAccountToken = pod.AutomountServiceAccountToken
				deletedPod.NodeName = pod.NodeName
				deletedPod.PodIP = pod.PodIP
				deletedPod.StartTime = pod.StartTime // NullTime
				deletedPod.RestartCount = pod.RestartCount
				deletedPod.OwnerKind = pod.OwnerKind
				deletedPod.OwnerName = pod.OwnerName
				deletedPod.ReplicaSetName = pod.ReplicaSetName
				deletedPod.QoSClass = pod.QoSClass
				deletedPod.SpecHash = pod.SpecHash
				deletedPod.DeletedAt = gorm.DeletedAt{}
				s.db.Save(&deletedPod)
				s.logger.Printf("🔄 Restored Pod %s/%s (SA: %s)", namespace, name, serviceAccount)
				// Ensure pod instance is active
				ctx := context.Background()
				if err := s.podInstanceManager.EnsureActiveInstance(ctx, uid, namespace, name); err != nil {
					s.logger.Printf("⚠️  Failed to ensure pod instance: %v", err)
				}
				// Restore: always run PCE (risk may be stale); pass current specHash for race protection
				go s.evaluatePodCapabilities(clusterID, uid, pod.SpecHash)
			} else {
				// Create new pod
				if err := s.db.Create(&pod).Error; err != nil {
					s.logger.Printf("❌ Failed to create Pod %s/%s: %v", namespace, name, err)
					continue
				}
				s.logger.Printf("✨ Created Pod %s/%s (SA: %s)", namespace, name, serviceAccount)
				// Ensure pod instance is active
				ctx := context.Background()
				if err := s.podInstanceManager.EnsureActiveInstance(ctx, uid, namespace, name); err != nil {
					s.logger.Printf("⚠️  Failed to ensure pod instance: %v", err)
				}
				go s.evaluatePodCapabilities(clusterID, uid, pod.SpecHash)
			}
		} else {
			// Database error
			s.logger.Printf("❌ DB error checking Pod %s/%s: %v", namespace, name, err)
			continue
		}
	}

	// Soft delete pods not in sync (for full sync only)
	// Also clean up duplicate pods (same UID, keep only the latest)
	if isFullSync && len(syncedUIDs) > 0 {
		keepUIDs := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			keepUIDs = append(keepUIDs, uid)
		}

		// Soft delete pods not in current sync
		var toDelete []models.Pod
		s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, keepUIDs).Find(&toDelete)

		for _, pod := range toDelete {
			s.db.Delete(&pod) // Soft delete
			// Clean Pod Detail data so DB does not keep orphaned process/metrics/network rows
			s.db.Where("pod_uid = ?", pod.UID).Delete(&models.PodProcess{})
			s.db.Where("pod_uid = ?", pod.UID).Delete(&models.PodRuntimeMetrics{})
			s.db.Where("pod_uid = ?", pod.UID).Delete(&models.PodNetworkConnection{})
			s.logger.Printf("🗑️  Soft-deleted Pod %s/%s (UID: %s) - not in full sync", pod.Namespace, pod.Name, pod.UID)
		}

		// Clean up duplicate pods (same UID) - keep only the latest one per UID
		for uid := range syncedUIDs {
			var duplicates []models.Pod
			s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).Order("created_at DESC").Find(&duplicates)

			if len(duplicates) > 1 {
				// Keep the first (latest) one, soft delete the rest
				for i := 1; i < len(duplicates); i++ {
					s.db.Delete(&duplicates[i])
					s.logger.Printf("🗑️  Soft-deleted duplicate Pod %s/%s (UID: %s) - keeping latest", duplicates[i].Namespace, duplicates[i].Name, uid)
				}
			}
		}
	}

	// Always clean up duplicates, even in delta sync (to prevent accumulation)
	if len(syncedUIDs) > 0 {
		for uid := range syncedUIDs {
			var duplicates []models.Pod
			s.db.Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", clusterID, uid).Order("created_at DESC").Find(&duplicates)

			if len(duplicates) > 1 {
				// Keep the first (latest) one, soft delete the rest
				for i := 1; i < len(duplicates); i++ {
					s.db.Delete(&duplicates[i])
					s.logger.Printf("🗑️  Soft-deleted duplicate Pod %s/%s (UID: %s) - keeping latest", duplicates[i].Namespace, duplicates[i].Name, uid)
				}
			}
		}
	}

	return nil
}

func (s *AgentService) evaluatePodCapabilities(clusterID, uid, specHash string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var pod models.Pod
	if err := s.db.WithContext(ctx).Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&pod).Error; err != nil {
		// Avoid noisy logs in tests: in-memory DB may be gone when goroutine runs after test exit
		if isTableMissingErr(err) {
			return
		}
		s.logger.Printf("⚠️  PCE skipped: pod not found (cluster=%s uid=%s): %v", clusterID, uid, err)
		return
	}
	if err := capability.EvaluateAndUpsertPod(ctx, s.db, &pod, specHash); err != nil {
		if isTableMissingErr(err) {
			return // e.g. test DB missing pod_capabilities/pods tables
		}
		s.logger.Printf("❌ PCE evaluation failed for pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}
}

// processSyncedDeployments handles Deployment sync
func (s *AgentService) processSyncedDeployments(clusterID string, data map[string]interface{}) error {
	deploymentsData, ok := data["deployments"].([]interface{})
	if !ok || len(deploymentsData) == 0 {
		s.logger.Printf("ℹ️  No deployments in payload")
		return nil
	}

	s.logger.Printf("📦 Processing %d deployments", len(deploymentsData))

	syncedUIDs := make(map[string]bool)

	for _, depData := range deploymentsData {
		depMap, ok := depData.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract basic fields
		name, nameOK := depMap["name"].(string)
		namespace, nsOK := depMap["namespace"].(string)
		uid, uidOK := depMap["uid"].(string)
		if !nameOK || !nsOK || !uidOK || uid == "" {
			continue
		}

		syncedUIDs[uid] = true

		// Extract replica counts
		replicas := int32(0)
		if r, ok := depMap["replicas"].(float64); ok {
			replicas = int32(r)
		}
		readyReplicas := int32(0)
		if r, ok := depMap["readyReplicas"].(float64); ok {
			readyReplicas = int32(r)
		}
		availableReplicas := int32(0)
		if r, ok := depMap["availableReplicas"].(float64); ok {
			availableReplicas = int32(r)
		}
		unavailableReplicas := int32(0)
		if r, ok := depMap["unavailableReplicas"].(float64); ok {
			unavailableReplicas = int32(r)
		}
		updatedReplicas := int32(0)
		if r, ok := depMap["updatedReplicas"].(float64); ok {
			updatedReplicas = int32(r)
		}

		// Extract strategy
		strategy := "RollingUpdate"
		if s, ok := depMap["strategy"].(string); ok {
			strategy = s
		}

		// Parse JSON fields
		labelsJSON := "{}"
		if labels, ok := depMap["labels"].(map[string]interface{}); ok {
			if bytes, err := json.Marshal(labels); err == nil {
				labelsJSON = string(bytes)
			}
		}

		annotationsJSON := "{}"
		if annotations, ok := depMap["annotations"].(map[string]interface{}); ok {
			if bytes, err := json.Marshal(annotations); err == nil {
				annotationsJSON = string(bytes)
			}
		}

		selectorJSON := "{}"
		if selector, ok := depMap["selector"].(map[string]interface{}); ok {
			if bytes, err := json.Marshal(selector); err == nil {
				selectorJSON = string(bytes)
			}
		}

		templateJSON := "{}"
		if containers, ok := depMap["containers"].([]interface{}); ok {
			template := map[string]interface{}{
				"containers": containers,
			}
			if bytes, err := json.Marshal(template); err == nil {
				templateJSON = string(bytes)
			}
		}

		conditionsJSON := "[]"
		if conditions, ok := depMap["conditions"].([]interface{}); ok {
			if bytes, err := json.Marshal(conditions); err == nil {
				conditionsJSON = string(bytes)
			}
		}

		deployment := models.Deployment{
			ClusterID:           clusterID,
			UID:                 uid,
			Name:                name,
			Namespace:           namespace,
			Replicas:            replicas,
			ReadyReplicas:       readyReplicas,
			AvailableReplicas:   availableReplicas,
			UnavailableReplicas: unavailableReplicas,
			UpdatedReplicas:     updatedReplicas,
			Strategy:            strategy,
			Labels:              labelsJSON,
			Annotations:         annotationsJSON,
			Selector:            selectorJSON,
			Template:            templateJSON,
			Conditions:          conditionsJSON,
		}

		// Upsert deployment
		var existing models.Deployment
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existing).Error
		if err == nil {
			// Check if changed
			changed := existing.Name != deployment.Name ||
				existing.Namespace != deployment.Namespace ||
				existing.Replicas != deployment.Replicas ||
				existing.ReadyReplicas != deployment.ReadyReplicas ||
				existing.AvailableReplicas != deployment.AvailableReplicas ||
				existing.Strategy != deployment.Strategy ||
				existing.Template != deployment.Template

			if changed {
				// Update existing
				s.db.Model(&existing).Updates(map[string]interface{}{
					"name":                 deployment.Name,
					"namespace":            deployment.Namespace,
					"replicas":             deployment.Replicas,
					"ready_replicas":       deployment.ReadyReplicas,
					"available_replicas":   deployment.AvailableReplicas,
					"unavailable_replicas": deployment.UnavailableReplicas,
					"updated_replicas":     deployment.UpdatedReplicas,
					"strategy":             deployment.Strategy,
					"labels":               deployment.Labels,
					"annotations":          deployment.Annotations,
					"selector":             deployment.Selector,
					"template":             deployment.Template,
					"conditions":           deployment.Conditions,
				})
				resourceID := strconv.Itoa(int(existing.ID))
				s.createAuditLog(clusterID, "update", "deployment", resourceID, namespace, name)
				s.logger.Printf("🔄 Updated Deployment %s/%s (ID=%d)", namespace, name, existing.ID)
			} else {
				s.logger.Printf("⏭️  Deployment %s/%s unchanged, skipping", namespace, name)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new
			if err := s.db.Create(&deployment).Error; err != nil {
				s.logger.Printf("❌ Failed to create Deployment %s/%s: %v", namespace, name, err)
				continue
			}
			resourceID := strconv.Itoa(int(deployment.ID))
			s.createAuditLog(clusterID, "create", "deployment", resourceID, namespace, name)
			s.logger.Printf("✨ Created Deployment %s/%s (ID=%d)", namespace, name, deployment.ID)
		} else {
			s.logger.Printf("❌ Error checking Deployment %s/%s: %v", namespace, name, err)
			continue
		}
	}

	// Delete deployments not in sync (removed from cluster)
	if len(syncedUIDs) > 0 {
		var toDelete []models.Deployment
		uidList := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			uidList = append(uidList, uid)
		}
		s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, uidList).Find(&toDelete)
		for _, dep := range toDelete {
			resourceID := strconv.Itoa(int(dep.ID))
			s.createAuditLog(clusterID, "delete", "deployment", resourceID, dep.Namespace, dep.Name)
			s.db.Delete(&dep)
			s.logger.Printf("🗑️  Deleted Deployment %s/%s (ID=%d) - not in full sync", dep.Namespace, dep.Name, dep.ID)
		}
	}

	return nil
}

// processSyncedReplicaSets handles ReplicaSet sync
func (s *AgentService) processSyncedReplicaSets(clusterID string, data map[string]interface{}) error {
	replicasetsData, ok := data["replicasets"].([]interface{})
	if !ok || len(replicasetsData) == 0 {
		s.logger.Printf("ℹ️  No replicasets in payload")
		return nil
	}

	s.logger.Printf("📦 Processing %d replicasets", len(replicasetsData))

	syncedUIDs := make(map[string]bool)

	for _, rsData := range replicasetsData {
		rsMap, ok := rsData.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract basic fields
		name, nameOK := rsMap["name"].(string)
		namespace, nsOK := rsMap["namespace"].(string)
		uid, uidOK := rsMap["uid"].(string)
		if !nameOK || !nsOK || !uidOK || uid == "" {
			continue
		}

		syncedUIDs[uid] = true

		// Extract replica counts
		replicas := int32(0)
		if r, ok := rsMap["replicas"].(float64); ok {
			replicas = int32(r)
		}
		readyReplicas := int32(0)
		if r, ok := rsMap["readyReplicas"].(float64); ok {
			readyReplicas = int32(r)
		}
		availableReplicas := int32(0)
		if r, ok := rsMap["availableReplicas"].(float64); ok {
			availableReplicas = int32(r)
		}
		fullyLabeledReplicas := int32(0)
		if r, ok := rsMap["fullyLabeledReplicas"].(float64); ok {
			fullyLabeledReplicas = int32(r)
		}

		// Extract owner reference
		ownerKind := ""
		if k, ok := rsMap["ownerKind"].(string); ok {
			ownerKind = k
		}
		ownerName := ""
		if n, ok := rsMap["ownerName"].(string); ok {
			ownerName = n
		}
		ownerUID := ""
		if u, ok := rsMap["ownerUid"].(string); ok {
			ownerUID = u
		}

		// Parse JSON fields
		labelsJSON := "{}"
		if labels, ok := rsMap["labels"].(map[string]interface{}); ok {
			if bytes, err := json.Marshal(labels); err == nil {
				labelsJSON = string(bytes)
			}
		}

		annotationsJSON := "{}"
		if annotations, ok := rsMap["annotations"].(map[string]interface{}); ok {
			if bytes, err := json.Marshal(annotations); err == nil {
				annotationsJSON = string(bytes)
			}
		}

		selectorJSON := "{}"
		if selector, ok := rsMap["selector"].(map[string]interface{}); ok {
			if bytes, err := json.Marshal(selector); err == nil {
				selectorJSON = string(bytes)
			}
		}

		templateJSON := "{}"
		if containers, ok := rsMap["containers"].([]interface{}); ok {
			template := map[string]interface{}{
				"containers": containers,
			}
			if bytes, err := json.Marshal(template); err == nil {
				templateJSON = string(bytes)
			}
		}

		conditionsJSON := "[]"
		if conditions, ok := rsMap["conditions"].([]interface{}); ok {
			if bytes, err := json.Marshal(conditions); err == nil {
				conditionsJSON = string(bytes)
			}
		}

		replicaset := models.ReplicaSet{
			ClusterID:            clusterID,
			UID:                  uid,
			Name:                 name,
			Namespace:            namespace,
			Replicas:             replicas,
			ReadyReplicas:        readyReplicas,
			AvailableReplicas:    availableReplicas,
			FullyLabeledReplicas: fullyLabeledReplicas,
			OwnerKind:            ownerKind,
			OwnerName:            ownerName,
			OwnerUID:             ownerUID,
			Labels:               labelsJSON,
			Annotations:          annotationsJSON,
			Selector:             selectorJSON,
			Template:             templateJSON,
			Conditions:           conditionsJSON,
		}

		// Upsert replicaset
		var existing models.ReplicaSet
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existing).Error
		if err == nil {
			// Check if changed
			changed := existing.Name != replicaset.Name ||
				existing.Namespace != replicaset.Namespace ||
				existing.Replicas != replicaset.Replicas ||
				existing.ReadyReplicas != replicaset.ReadyReplicas ||
				existing.AvailableReplicas != replicaset.AvailableReplicas ||
				existing.FullyLabeledReplicas != replicaset.FullyLabeledReplicas ||
				existing.OwnerKind != replicaset.OwnerKind ||
				existing.OwnerName != replicaset.OwnerName ||
				existing.Template != replicaset.Template

			if changed {
				// Update existing
				s.db.Model(&existing).Updates(map[string]interface{}{
					"name":                   replicaset.Name,
					"namespace":              replicaset.Namespace,
					"replicas":               replicaset.Replicas,
					"ready_replicas":         replicaset.ReadyReplicas,
					"available_replicas":     replicaset.AvailableReplicas,
					"fully_labeled_replicas": replicaset.FullyLabeledReplicas,
					"owner_kind":             replicaset.OwnerKind,
					"owner_name":             replicaset.OwnerName,
					"owner_uid":              replicaset.OwnerUID,
					"labels":                 replicaset.Labels,
					"annotations":            replicaset.Annotations,
					"selector":               replicaset.Selector,
					"template":               replicaset.Template,
					"conditions":             replicaset.Conditions,
				})
				resourceID := strconv.Itoa(int(existing.ID))
				s.createAuditLog(clusterID, "update", "replicaset", resourceID, namespace, name)
				s.logger.Printf("🔄 Updated ReplicaSet %s/%s (ID=%d)", namespace, name, existing.ID)
			} else {
				s.logger.Printf("⏭️  ReplicaSet %s/%s unchanged, skipping", namespace, name)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new
			if err := s.db.Create(&replicaset).Error; err != nil {
				s.logger.Printf("❌ Failed to create ReplicaSet %s/%s: %v", namespace, name, err)
				continue
			}
			resourceID := strconv.Itoa(int(replicaset.ID))
			s.createAuditLog(clusterID, "create", "replicaset", resourceID, namespace, name)
			s.logger.Printf("✨ Created ReplicaSet %s/%s (ID=%d)", namespace, name, replicaset.ID)
		} else {
			s.logger.Printf("❌ Error checking ReplicaSet %s/%s: %v", namespace, name, err)
			continue
		}
	}

	// Delete replicasets not in sync (removed from cluster)
	if len(syncedUIDs) > 0 {
		var toDelete []models.ReplicaSet
		uidList := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			uidList = append(uidList, uid)
		}
		s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, uidList).Find(&toDelete)
		for _, rs := range toDelete {
			resourceID := strconv.Itoa(int(rs.ID))
			s.createAuditLog(clusterID, "delete", "replicaset", resourceID, rs.Namespace, rs.Name)
			s.db.Delete(&rs)
			s.logger.Printf("🗑️  Deleted ReplicaSet %s/%s (ID=%d) - not in full sync", rs.Namespace, rs.Name, rs.ID)
		}
	}

	return nil
}

// cleanupOldAuditLogs deletes audit logs older than TTL
func (s *AgentService) cleanupOldAuditLogs() {
	cutoff := time.Now().Add(-auditLogTTL)
	result := s.db.Where("created_at < ?", cutoff).Delete(&models.AuditLog{})
	if result.Error != nil {
		s.logger.Printf("❌ Failed to cleanup old audit logs: %v", result.Error)
	} else if result.RowsAffected > 0 {
		s.logger.Printf("🧹 Cleaned up %d old audit logs", result.RowsAffected)
	}
}

// StaleClusterCutoff: clusters not synced in this duration are soft-deleted so dashboard doesn't show old env data.
const StaleClusterCutoff = 90 * 24 * time.Hour

func (s *AgentService) cleanupStaleClusters() {
	cutoff := time.Now().Add(-StaleClusterCutoff)
	var toDelete []models.Cluster
	if err := s.db.Unscoped().Where("last_sync < ? AND deleted_at IS NULL", cutoff).Find(&toDelete).Error; err != nil {
		s.logger.Printf("❌ Failed to list stale clusters: %v", err)
		return
	}
	for _, c := range toDelete {
		// Soft-delete pods (and other cluster-scoped resources) so dashboard doesn't show old pod data
		s.db.Where("cluster_id = ?", c.ID).Delete(&models.Pod{})
		if err := s.db.Delete(&c).Error; err != nil {
			s.logger.Printf("❌ Failed to soft-delete stale cluster %s: %v", c.ID, err)
			continue
		}
		s.logger.Printf("🧹 Soft-deleted stale cluster %s (last_sync %v)", c.ID, c.LastSync)
	}
}

func (s *AgentService) cleanupStalePods(clusterID string) {
	cutoff := time.Now().Add(-getStalePodCutoff())
	var stale []models.Pod
	if err := s.db.Where("cluster_id = ? AND deleted_at IS NULL AND updated_at < ?", clusterID, cutoff).Find(&stale).Error; err != nil {
		s.logger.Printf("❌ Failed to list stale pods for cluster %s: %v", clusterID, err)
		return
	}
	if len(stale) == 0 {
		return
	}
	for _, p := range stale {
		if err := s.db.Delete(&p).Error; err != nil {
			s.logger.Printf("❌ Failed to soft-delete stale pod %s/%s (%s): %v", p.Namespace, p.Name, p.UID, err)
			continue
		}
		s.logger.Printf("🧹 Soft-deleted stale pod %s/%s (%s), updated_at=%s", p.Namespace, p.Name, p.UID, p.UpdatedAt.Format(time.RFC3339))
	}
}

// GetAgentHandler returns gin handler for agent sync
func GetAgentHandler(db *gorm.DB) gin.HandlerFunc {
	service := NewAgentService(db)
	return func(c *gin.Context) {
		var req struct {
			ClusterID   string                 `json:"clusterId"`
			ClusterName string                 `json:"clusterName"`
			Data        map[string]interface{} `json:"data"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body", "details": err.Error()})
			return
		}

		if req.ClusterID == "" {
			c.JSON(400, gin.H{"error": "clusterId is required"})
			return
		}

		if err := service.SyncData(req.ClusterID, req.ClusterName, "", "", "", req.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to sync data", "details": err.Error()})
			return
		}

		c.JSON(200, gin.H{"success": true, "message": "Data synced successfully"})
	}
}
