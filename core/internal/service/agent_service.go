package service

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
)

const (
	// Audit log batch size
	auditLogBatchSize = 100
	// Audit log TTL - delete logs older than 90 days
	auditLogTTLDays = 90
	// Minimum time between audit logs for same resource (to avoid spam)
	minAuditLogInterval = 10 * time.Second
)

// getMapKeys returns all keys from a map
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// extractBoolFlag attempts to read a boolean value from map data
func extractBoolFlag(data map[string]interface{}, key string) (bool, bool, string) {
	val, ok := data[key]
	if !ok {
		return false, false, "missing"
	}

	switch v := val.(type) {
	case bool:
		return v, true, "bool"
	case float64:
		return v != 0, true, "float64"
	case string:
		lower := strings.ToLower(strings.TrimSpace(v))
		return lower == "true" || lower == "1", true, "string"
	default:
		return false, false, "unsupported"
	}
}

// AgentService handles data from agents
type AgentService struct {
	db             *gorm.DB
	auditLogBuffer []models.AuditLog
	bufferMutex    sync.Mutex
	lastFullSync   time.Time
	isFirstSync    bool
	logger         *log.Logger
}

// NewAgentService creates a new AgentService
func NewAgentService(db *gorm.DB) *AgentService {
	var writer io.Writer
	if gin.DefaultWriter != nil {
		writer = io.MultiWriter(os.Stdout, gin.DefaultWriter)
	} else {
		writer = os.Stdout
	}

	return &AgentService{
		db:             db,
		auditLogBuffer: make([]models.AuditLog, 0, auditLogBatchSize),
		isFirstSync:    true,
		logger:         log.New(writer, "[AgentService] ", log.LstdFlags|log.Lmicroseconds),
	}
}

// addAuditLogToBuffer adds audit log to buffer for batch insert
func (s *AgentService) addAuditLogToBuffer(auditLog models.AuditLog) {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	s.auditLogBuffer = append(s.auditLogBuffer, auditLog)

	// Flush buffer if it reaches batch size
	if len(s.auditLogBuffer) >= auditLogBatchSize {
		s.flushAuditLogBuffer()
	}
}

// flushAuditLogBuffer flushes audit log buffer to database
func (s *AgentService) flushAuditLogBuffer() {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	if len(s.auditLogBuffer) == 0 {
		return
	}

	// Batch insert
	if err := s.db.CreateInBatches(s.auditLogBuffer, auditLogBatchSize).Error; err != nil {
		s.logger.Printf("Error batch inserting audit logs: %v", err)
	} else {
		s.logger.Printf("Batch inserted %d audit logs", len(s.auditLogBuffer))
	}

	// Clear buffer
	s.auditLogBuffer = s.auditLogBuffer[:0]
}

// shouldCreateAuditLog checks if audit log should be created (avoid spam)
func (s *AgentService) shouldCreateAuditLog(resource, resourceID, action, user string) bool {
	// Always allow create/delete events (no throttling) per architecture doc
	if action == "create" || action == "delete" {
		return true
	}

	var existingLog models.AuditLog
	err := s.db.
		Where("resource = ? AND resource_id = ? AND action = ? AND \"user\" = ? AND created_at > ?",
			resource, resourceID, action, user, time.Now().Add(-minAuditLogInterval)).
		First(&existingLog).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	if err != nil {
		s.logger.Printf("shouldCreateAuditLog: error checking recent logs (resource=%s, id=%s, action=%s): %v",
			resource, resourceID, action, err)
		return true
	}
	return false
}

// cleanupOldAuditLogs deletes audit logs older than TTL
func (s *AgentService) cleanupOldAuditLogs() error {
	cutoffDate := time.Now().AddDate(0, 0, -auditLogTTLDays)
	result := s.db.Where("created_at < ?", cutoffDate).Delete(&models.AuditLog{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		s.logger.Printf("Cleaned up %d old audit logs (older than %d days)", result.RowsAffected, auditLogTTLDays)
	}
	return nil
}

// SyncData syncs collected data from agent
func (s *AgentService) SyncData(clusterID string, data map[string]interface{}) error {
	// Update or create cluster
	cluster := models.Cluster{
		ID:       clusterID,
		Name:     clusterID, // You might want to get this from data
		Status:   "active",
		LastSync: time.Now(),
	}
	s.db.Save(&cluster)

	// Determine sync type based on explicit flags first, then fall back to heuristics
	hasRoleBindings := data["roleBindings"] != nil
	hasClusterRoleBindings := data["clusterRoleBindings"] != nil
	hasRoles := data["roles"] != nil

	// Default inference (used only if flags are missing)
	inferredFullSync := len(data) >= 4 && hasRoleBindings && hasClusterRoleBindings && hasRoles

	isFullSync := inferredFullSync
	fullFlagProvided := false
	if flagVal, ok, source := extractBoolFlag(data, "isFullSync"); ok {
		isFullSync = flagVal
		fullFlagProvided = true
		s.logger.Printf("SyncData: isFullSync = %v (source=%s)", isFullSync, source)
	} else {
		s.logger.Printf("SyncData: isFullSync flag missing, inferred=%v (hasRoleBindings=%v, hasClusterRoleBindings=%v, hasRoles=%v)",
			isFullSync, hasRoleBindings, hasClusterRoleBindings, hasRoles)
	}

	// isDeltaSync flag (independent of heuristics)
	isDeltaSync := false
	if flagVal, ok, source := extractBoolFlag(data, "isDeltaSync"); ok {
		isDeltaSync = flagVal
		s.logger.Printf("SyncData: isDeltaSync = %v (source=%s)", isDeltaSync, source)
	} else {
		// If delta flag missing but full sync flag provided, assume delta is inverse
		if fullFlagProvided {
			isDeltaSync = !isFullSync
			s.logger.Printf("SyncData: isDeltaSync not provided, defaulting to %v based on isFullSync flag", isDeltaSync)
		} else {
			isDeltaSync = !isFullSync && !inferredFullSync
			s.logger.Printf("SyncData: isDeltaSync not provided, defaulting to %v (inferred)", isDeltaSync)
		}
	}

	// Debug: Log data keys for troubleshooting
	s.logger.Printf("SyncData: Processing sync for cluster %s, isFullSync=%v, isDeltaSync=%v, data keys: %v",
		clusterID, isFullSync, isDeltaSync, getMapKeys(data))

	// Cleanup old audit logs periodically (every hour)
	if time.Since(s.lastFullSync) > time.Hour {
		s.cleanupOldAuditLogs()
		s.lastFullSync = time.Now()
	}

	// Flush audit log buffer at the end
	defer s.flushAuditLogBuffer()

	// Get system user ID once for all audit logs in this sync
	var systemUserID uint = 1 // Default to admin user
	var systemUser models.User
	if err := s.db.Where("username = ? OR id = ?", "system", 0).First(&systemUser).Error; err == nil {
		systemUserID = systemUser.ID
	} else {
		// Fallback: use first admin user or user ID 1
		if err := s.db.Where("role = ?", "admin").First(&systemUser).Error; err == nil {
			systemUserID = systemUser.ID
		}
	}

	// Sync ServiceAccounts
	syncedUIDs := make(map[string]bool)

	// Track ServiceAccounts that exist in DB before sync
	var existingSAs []models.ServiceAccount
	s.db.Where("cluster_id = ?", clusterID).Find(&existingSAs)
	existingUIDs := make(map[string]uint) // uid -> id mapping
	for _, sa := range existingSAs {
		existingUIDs[sa.UID] = sa.ID
	}

	if sas, ok := data["serviceAccounts"].([]interface{}); ok {
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

			sa := models.ServiceAccount{
				ClusterID: clusterID,
				Name:      name,
				Namespace: namespace,
				UID:       uid,
			}

			// Handle labels - ensure valid JSON
			if labels, ok := saMap["labels"].(map[string]interface{}); ok && labels != nil {
				if labelsJSON, err := json.Marshal(labels); err == nil {
					sa.Labels = string(labelsJSON)
				} else {
					sa.Labels = "{}"
				}
			} else {
				sa.Labels = "{}"
			}

			// Handle secrets - ensure valid JSON array
			if secrets, ok := saMap["secrets"].([]interface{}); ok && secrets != nil {
				if secretsJSON, err := json.Marshal(secrets); err == nil {
					sa.Secrets = string(secretsJSON)
				} else {
					sa.Secrets = "[]"
				}
			} else {
				sa.Secrets = "[]"
			}

			syncedUIDs[uid] = true

			var existingActiveSA models.ServiceAccount
			activeResult := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existingActiveSA)

			if activeResult.Error == nil {
				// Update existing ServiceAccount
				// Check if this is actually a new ServiceAccount (created very recently, within 2 minutes)
				// If so, create "create" audit log instead of "update"
				isNewSA := existingActiveSA.CreatedAt.After(time.Now().Add(-2 * time.Minute))

				existingActiveSA.Name = sa.Name
				existingActiveSA.Namespace = sa.Namespace
				existingActiveSA.Labels = sa.Labels
				existingActiveSA.Secrets = sa.Secrets
				if err := s.db.Save(&existingActiveSA).Error; err != nil {
					return err
				}

				// For delta sync from watcher, always create "create" audit log for new SAs
				// For full sync, only create audit log if it's a new SA and no create log exists
				resourceID := strconv.Itoa(int(existingActiveSA.ID))
				action := "update"

				// Check if create audit log already exists
				var existingLog models.AuditLog
				hasCreateLog := s.db.Where("resource = ? AND resource_id = ? AND action = ? AND \"user\" = ? AND created_at > NOW() - INTERVAL '5 minutes'",
					"serviceaccount", resourceID, "create", "system").First(&existingLog).Error == nil

				// For delta sync from watcher: if no create log exists, create one
				// This handles the case when watcher detects a new SA but it already exists in DB (from collector)
				if isDeltaSync && !hasCreateLog {
					action = "create"
					s.logger.Printf("SyncData: Setting action=create for SA %s/%s (ID: %s) - isDeltaSync=true, hasCreateLog=false", sa.Namespace, sa.Name, resourceID)
				} else if isNewSA && !hasCreateLog {
					// For full sync: only create if SA is new and no create log exists
					action = "create"
					s.logger.Printf("SyncData: Setting action=create for SA %s/%s (ID: %s) - isNewSA=true, hasCreateLog=false", sa.Namespace, sa.Name, resourceID)
				} else {
					s.logger.Printf("SyncData: Keeping action=update for SA %s/%s (ID: %s) - isDeltaSync=%v, isNewSA=%v, hasCreateLog=%v", sa.Namespace, sa.Name, resourceID, isDeltaSync, isNewSA, hasCreateLog)
				}

				// For delta sync (from watcher), always create audit log for new SAs (create action)
				// For full sync or update, use shouldCreateAuditLog to avoid spam
				shouldLog := false
				if isDeltaSync && action == "create" {
					// Delta sync from watcher: always log new SAs
					shouldLog = true
				} else {
					// Full sync or update: use shouldCreateAuditLog to avoid spam
					if action == "create" {
						shouldLog = s.shouldCreateAuditLog("serviceaccount", resourceID, "create", "system")
					} else if action == "update" {
						shouldLog = s.shouldCreateAuditLog("serviceaccount", resourceID, "update", "system")
					}
				}

				if shouldLog {
					auditLog := models.AuditLog{
						ClusterID:  clusterID,
						UserID:     systemUserID, // System/Agent action
						Action:     action,
						Resource:   "serviceaccount",
						ResourceID: resourceID,
						User:       "system",
						IP:         "agent-sync",
						Details:    `{"source":"agent-sync","namespace":"` + sa.Namespace + `","name":"` + sa.Name + `"}`,
					}
					s.addAuditLogToBuffer(auditLog)
					if isDeltaSync {
						s.flushAuditLogBuffer()
					}
					s.logger.Printf("Added audit log for %s SA: %s/%s (ID: %s, isDeltaSync: %v, isNewSA: %v)", action, sa.Namespace, sa.Name, resourceID, isDeltaSync, isNewSA)
				} else {
					s.logger.Printf("Skipped audit log for %s SA: %s/%s (ID: %s) - recent log exists or not new", action, sa.Namespace, sa.Name, resourceID)
				}
				continue
			}

			if !errors.Is(activeResult.Error, gorm.ErrRecordNotFound) {
				return activeResult.Error
			}

			var deletedSA models.ServiceAccount
			deletedResult := s.db.Unscoped().Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&deletedSA)

			if deletedResult.Error == nil {
				// Restore deleted ServiceAccount
				deletedSA.Name = sa.Name
				deletedSA.Namespace = sa.Namespace
				deletedSA.Labels = sa.Labels
				deletedSA.Secrets = sa.Secrets
				if deletedSA.DeletedAt.Valid {
					deletedSA.DeletedAt = gorm.DeletedAt{}
				}
				if err := s.db.Save(&deletedSA).Error; err != nil {
					return err
				}

				// Create audit log for restore (only if should create)
				resourceID := strconv.Itoa(int(deletedSA.ID))
				if s.shouldCreateAuditLog("serviceaccount", resourceID, "create", "system") {
					auditLog := models.AuditLog{
						ClusterID:  clusterID,
						UserID:     systemUserID, // System/Agent action
						Action:     "create",
						Resource:   "serviceaccount",
						ResourceID: resourceID,
						User:       "system",
						IP:         "agent-sync",
						Details:    `{"source":"agent-sync","namespace":"` + sa.Namespace + `","name":"` + sa.Name + `","action":"restored"}`,
					}
					s.addAuditLogToBuffer(auditLog)
					if isDeltaSync {
						s.flushAuditLogBuffer()
					}
				}
			} else if errors.Is(deletedResult.Error, gorm.ErrRecordNotFound) {
				// Create new ServiceAccount
				if err := s.db.Create(&sa).Error; err != nil {
					return err
				}

				// Create audit log for create
				// For delta sync (from watcher), always create audit log for new SAs
				// For full sync, use shouldCreateAuditLog to avoid spam
				resourceID := strconv.Itoa(int(sa.ID))
				shouldLog := false
				if isDeltaSync {
					// Delta sync from watcher: always log new SAs
					shouldLog = true
				} else {
					// Full sync: use shouldCreateAuditLog to avoid spam
					shouldLog = s.shouldCreateAuditLog("serviceaccount", resourceID, "create", "system")
				}

				if shouldLog {
					auditLog := models.AuditLog{
						ClusterID:  clusterID,
						UserID:     systemUserID, // System/Agent action
						Action:     "create",
						Resource:   "serviceaccount",
						ResourceID: resourceID,
						User:       "system",
						IP:         "agent-sync",
						Details:    `{"source":"agent-sync","namespace":"` + sa.Namespace + `","name":"` + sa.Name + `"}`,
					}
					s.addAuditLogToBuffer(auditLog)
					if isDeltaSync {
						s.flushAuditLogBuffer()
					}
					s.logger.Printf("Added audit log for new SA: %s/%s (ID: %s, isDeltaSync: %v)", sa.Namespace, sa.Name, resourceID, isDeltaSync)
				} else {
					s.logger.Printf("Skipped audit log for new SA: %s/%s (ID: %s) - recent log exists", sa.Namespace, sa.Name, resourceID)
				}
			} else {
				return deletedResult.Error
			}
		}
	}

	// Handle deletions:
	// 1. Full sync: delete ServiceAccounts that no longer exist in K8s
	// 2. Delta sync from watcher: when watcher detects delete, it sends SA data but SA no longer exists in K8s
	//    We need to detect this by checking if SA exists in DB but not in current sync data

	// For delta sync delete events from watcher:
	// When watcher detects a delete, it sends the SA data one last time.
	// We can detect this by checking if this is a delta sync with only ServiceAccounts
	// and the SA exists in DB - in this case, we should delete it from DB
	if isDeltaSync && len(syncedUIDs) > 0 {
		// This is a delta sync from watcher
		// Check if any SAs in sync data exist in DB - if so, they might be delete events
		// Actually, watcher sends SA data even for delete events, so we can't distinguish
		// The real deletion detection happens during full sync below
		// But we can still create audit logs for deletions detected during full sync
	}

	if isFullSync && len(syncedUIDs) > 0 {
		// Convert map to slice for batch delete query
		keepUIDs := make([]string, 0, len(syncedUIDs))
		for uid := range syncedUIDs {
			keepUIDs = append(keepUIDs, uid)
		}

		// Get ServiceAccounts that will be deleted (including already soft-deleted ones)
		var toDelete []models.ServiceAccount
		s.db.Unscoped().Where("cluster_id = ? AND uid NOT IN ?", clusterID, keepUIDs).Find(&toDelete)

		// Batch delete: remove ServiceAccounts that exist in DB but not in sync data
		// Only execute if we have UIDs to keep (safety check)
		if len(keepUIDs) > 0 {
			// Find ServiceAccounts that need audit logs (not yet logged)
			var needAuditLog []models.ServiceAccount
			for _, sa := range toDelete {
				// Check if audit log already exists for this deletion
				var existingLog models.AuditLog
				if err := s.db.Where("resource = ? AND resource_id = ? AND action = ? AND \"user\" = ? AND created_at > NOW() - INTERVAL '1 hour'",
					"serviceaccount", strconv.Itoa(int(sa.ID)), "delete", "system").First(&existingLog).Error; err != nil {
					// No audit log found, need to create one
					needAuditLog = append(needAuditLog, sa)
				}
			}

			// Create audit logs for deleted ServiceAccounts that don't have one yet (batch)
			for _, deletedSA := range needAuditLog {
				resourceID := strconv.Itoa(int(deletedSA.ID))
				if s.shouldCreateAuditLog("serviceaccount", resourceID, "delete", "system") {
					auditLog := models.AuditLog{
						ClusterID:  clusterID,
						UserID:     systemUserID, // System/Agent action
						Action:     "delete",
						Resource:   "serviceaccount",
						ResourceID: resourceID,
						User:       "system",
						IP:         "agent-sync",
						Details:    `{"source":"agent-sync","namespace":"` + deletedSA.Namespace + `","name":"` + deletedSA.Name + `"}`,
					}
					s.addAuditLogToBuffer(auditLog)
				}
			}

			// Actually delete ServiceAccounts that are not in sync data
			result := s.db.Where("cluster_id = ? AND uid NOT IN ?", clusterID, keepUIDs).
				Delete(&models.ServiceAccount{})

			s.logger.Printf("Full sync: Deleted %d ServiceAccounts, created %d audit logs", result.RowsAffected, len(needAuditLog))
		}
	}

	// Sync RoleBindings
	if rbs, ok := data["roleBindings"].([]interface{}); ok {
		for _, rbData := range rbs {
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

			rb := models.RoleBinding{
				ClusterID: clusterID,
				Name:      name,
				Namespace: namespace,
				UID:       uid,
			}

			if roleRef, ok := rbMap["roleRef"].(map[string]interface{}); ok && roleRef != nil {
				if roleRefJSON, err := json.Marshal(roleRef); err == nil {
					rb.RoleRef = string(roleRefJSON)
				} else {
					rb.RoleRef = "{}"
				}
			} else {
				rb.RoleRef = "{}"
			}

			if subjects, ok := rbMap["subjects"].([]interface{}); ok && subjects != nil {
				if subjectsJSON, err := json.Marshal(subjects); err == nil {
					rb.Subjects = string(subjectsJSON)
				} else {
					rb.Subjects = "[]"
				}
			} else {
				rb.Subjects = "[]"
			}

			var existingRB models.RoleBinding
			activeResult := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existingRB)

			if activeResult.Error == nil {
				existingRB.Name = rb.Name
				existingRB.Namespace = rb.Namespace
				existingRB.RoleRef = rb.RoleRef
				existingRB.Subjects = rb.Subjects
				if err := s.db.Save(&existingRB).Error; err != nil {
					return err
				}
				continue
			}

			if !errors.Is(activeResult.Error, gorm.ErrRecordNotFound) {
				return activeResult.Error
			}

			var deletedRB models.RoleBinding
			deletedResult := s.db.Unscoped().Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&deletedRB)

			if deletedResult.Error == nil {
				deletedRB.Name = rb.Name
				deletedRB.Namespace = rb.Namespace
				deletedRB.RoleRef = rb.RoleRef
				deletedRB.Subjects = rb.Subjects
				if deletedRB.DeletedAt.Valid {
					deletedRB.DeletedAt = gorm.DeletedAt{}
				}
				if err := s.db.Save(&deletedRB).Error; err != nil {
					return err
				}
			} else if errors.Is(deletedResult.Error, gorm.ErrRecordNotFound) {
				if err := s.db.Create(&rb).Error; err != nil {
					return err
				}
			} else {
				return deletedResult.Error
			}
		}
	}

	// Sync ClusterRoleBindings
	if crbs, ok := data["clusterRoleBindings"].([]interface{}); ok {
		for _, crbData := range crbs {
			crbMap, ok := crbData.(map[string]interface{})
			if !ok {
				continue
			}

			name, nameOK := crbMap["name"].(string)
			uid, uidOK := crbMap["uid"].(string)
			if !nameOK || !uidOK || uid == "" {
				continue
			}

			crb := models.ClusterRoleBinding{
				ClusterID: clusterID,
				Name:      name,
				UID:       uid,
			}

			if roleRef, ok := crbMap["roleRef"].(map[string]interface{}); ok && roleRef != nil {
				if roleRefJSON, err := json.Marshal(roleRef); err == nil {
					crb.RoleRef = string(roleRefJSON)
				} else {
					crb.RoleRef = "{}"
				}
			} else {
				crb.RoleRef = "{}"
			}

			if subjects, ok := crbMap["subjects"].([]interface{}); ok && subjects != nil {
				if subjectsJSON, err := json.Marshal(subjects); err == nil {
					crb.Subjects = string(subjectsJSON)
				} else {
					crb.Subjects = "[]"
				}
			} else {
				crb.Subjects = "[]"
			}

			var existingCRB models.ClusterRoleBinding
			activeResult := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existingCRB)

			if activeResult.Error == nil {
				existingCRB.Name = crb.Name
				existingCRB.RoleRef = crb.RoleRef
				existingCRB.Subjects = crb.Subjects
				if err := s.db.Save(&existingCRB).Error; err != nil {
					return err
				}
				continue
			}

			if !errors.Is(activeResult.Error, gorm.ErrRecordNotFound) {
				return activeResult.Error
			}

			var deletedCRB models.ClusterRoleBinding
			deletedResult := s.db.Unscoped().Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&deletedCRB)

			if deletedResult.Error == nil {
				deletedCRB.Name = crb.Name
				deletedCRB.RoleRef = crb.RoleRef
				deletedCRB.Subjects = crb.Subjects
				if deletedCRB.DeletedAt.Valid {
					deletedCRB.DeletedAt = gorm.DeletedAt{}
				}
				if err := s.db.Save(&deletedCRB).Error; err != nil {
					return err
				}
			} else if errors.Is(deletedResult.Error, gorm.ErrRecordNotFound) {
				if err := s.db.Create(&crb).Error; err != nil {
					return err
				}
			} else {
				return deletedResult.Error
			}
		}
	}

	// Sync Roles
	if roles, ok := data["roles"].([]interface{}); ok {
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

			role := models.Role{
				ClusterID: clusterID,
				Name:      name,
				Namespace: namespace,
				UID:       uid,
			}

			if rules, ok := roleMap["rules"].([]interface{}); ok && rules != nil {
				if rulesJSON, err := json.Marshal(rules); err == nil {
					role.Rules = string(rulesJSON)
				} else {
					role.Rules = "[]"
				}
			} else {
				role.Rules = "[]"
			}

			var existingRole models.Role
			activeResult := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existingRole)

			if activeResult.Error == nil {
				existingRole.Name = role.Name
				existingRole.Namespace = role.Namespace
				existingRole.Rules = role.Rules
				if err := s.db.Save(&existingRole).Error; err != nil {
					return err
				}
				continue
			}

			if !errors.Is(activeResult.Error, gorm.ErrRecordNotFound) {
				return activeResult.Error
			}

			var deletedRole models.Role
			deletedResult := s.db.Unscoped().Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&deletedRole)

			if deletedResult.Error == nil {
				deletedRole.Name = role.Name
				deletedRole.Namespace = role.Namespace
				deletedRole.Rules = role.Rules
				if deletedRole.DeletedAt.Valid {
					deletedRole.DeletedAt = gorm.DeletedAt{}
				}
				if err := s.db.Save(&deletedRole).Error; err != nil {
					return err
				}
			} else if errors.Is(deletedResult.Error, gorm.ErrRecordNotFound) {
				if err := s.db.Create(&role).Error; err != nil {
					return err
				}
			} else {
				return deletedResult.Error
			}
		}
	}

	// Sync ClusterRoles
	if crs, ok := data["clusterRoles"].([]interface{}); ok {
		for _, crData := range crs {
			crMap, ok := crData.(map[string]interface{})
			if !ok {
				continue
			}

			name, nameOK := crMap["name"].(string)
			uid, uidOK := crMap["uid"].(string)
			if !nameOK || !uidOK || uid == "" {
				continue
			}

			cr := models.ClusterRole{
				ClusterID: clusterID,
				Name:      name,
				UID:       uid,
			}

			if rules, ok := crMap["rules"].([]interface{}); ok && rules != nil {
				if rulesJSON, err := json.Marshal(rules); err == nil {
					cr.Rules = string(rulesJSON)
				} else {
					cr.Rules = "[]"
				}
			} else {
				cr.Rules = "[]"
			}

			var existingCR models.ClusterRole
			activeResult := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existingCR)

			if activeResult.Error == nil {
				existingCR.Name = cr.Name
				existingCR.Rules = cr.Rules
				if err := s.db.Save(&existingCR).Error; err != nil {
					return err
				}
				continue
			}

			if !errors.Is(activeResult.Error, gorm.ErrRecordNotFound) {
				return activeResult.Error
			}

			var deletedCR models.ClusterRole
			deletedResult := s.db.Unscoped().Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&deletedCR)

			if deletedResult.Error == nil {
				deletedCR.Name = cr.Name
				deletedCR.Rules = cr.Rules
				if deletedCR.DeletedAt.Valid {
					deletedCR.DeletedAt = gorm.DeletedAt{}
				}
				if err := s.db.Save(&deletedCR).Error; err != nil {
					return err
				}
			} else if errors.Is(deletedResult.Error, gorm.ErrRecordNotFound) {
				if err := s.db.Create(&cr).Error; err != nil {
					return err
				}
			} else {
				return deletedResult.Error
			}
		}
	}

	// Sync Pods
	if pods, ok := data["pods"].([]interface{}); ok {
		for _, podData := range pods {
			podMap, ok := podData.(map[string]interface{})
			if !ok {
				continue
			}

			name, nameOK := podMap["name"].(string)
			namespace, nsOK := podMap["namespace"].(string)
			serviceAccount, saOK := podMap["serviceAccount"].(string)
			uid, uidOK := podMap["uid"].(string)
			if !nameOK || !nsOK || !saOK || !uidOK || uid == "" {
				continue
			}

			pod := models.Pod{
				ClusterID:      clusterID,
				Name:           name,
				Namespace:      namespace,
				ServiceAccount: serviceAccount,
				UID:            uid,
			}

			var existingPod models.Pod
			activeResult := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existingPod)

			if activeResult.Error == nil {
				existingPod.Name = pod.Name
				existingPod.Namespace = pod.Namespace
				existingPod.ServiceAccount = pod.ServiceAccount
				if err := s.db.Save(&existingPod).Error; err != nil {
					return err
				}
				continue
			}

			if !errors.Is(activeResult.Error, gorm.ErrRecordNotFound) {
				return activeResult.Error
			}

			var deletedPod models.Pod
			deletedResult := s.db.Unscoped().Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&deletedPod)

			if deletedResult.Error == nil {
				deletedPod.Name = pod.Name
				deletedPod.Namespace = pod.Namespace
				deletedPod.ServiceAccount = pod.ServiceAccount
				if deletedPod.DeletedAt.Valid {
					deletedPod.DeletedAt = gorm.DeletedAt{}
				}
				if err := s.db.Save(&deletedPod).Error; err != nil {
					return err
				}
			} else if errors.Is(deletedResult.Error, gorm.ErrRecordNotFound) {
				if err := s.db.Create(&pod).Error; err != nil {
					return err
				}
			} else {
				return deletedResult.Error
			}
		}
	}

	return nil
}
