package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/internal/auth"
	"github.com/ksam/core/pkg/models"
)

const (
	// Audit log TTL - delete logs older than 90 days
	auditLogTTL = 90 * 24 * time.Hour
)

// AgentService handles data from agents
type AgentService struct {
	db             *gorm.DB
	logger         *log.Logger
	systemUserID   uint
	systemUserOnce sync.Once
}

// NewAgentService creates a new agent service
func NewAgentService(db *gorm.DB) *AgentService {
	writer := io.MultiWriter(os.Stdout)
	return &AgentService{
		db:     db,
		logger: log.New(writer, "[AgentService] ", log.LstdFlags|log.Lmicroseconds),
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
		Email:    "system@ksam.local",
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

// SyncData syncs collected data from agent
func (s *AgentService) SyncData(clusterID string, data map[string]interface{}) error {
	// Update or create cluster
	cluster := models.Cluster{
		ID:       clusterID,
		Name:     clusterID,
		Status:   "active",
		LastSync: time.Now(),
	}
	s.db.Save(&cluster)

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

	// Cleanup old audit logs (TTL)
	go s.cleanupOldAuditLogs()

	return nil
}

// processSyncedServiceAccounts handles ServiceAccount sync
func (s *AgentService) processSyncedServiceAccounts(clusterID string, data map[string]interface{}, isFullSync, isDeltaSync bool) error {
	sas, ok := data["serviceAccounts"].([]interface{})
	if !ok || len(sas) == 0 {
		s.logger.Printf("ℹ️  No serviceAccounts in payload")
		// For full sync with no SAs, delete all existing SAs
		if isFullSync {
			var existingSAs []models.ServiceAccount
			s.db.Where("cluster_id = ?", clusterID).Find(&existingSAs)
			for _, sa := range existingSAs {
				resourceID := strconv.Itoa(int(sa.ID))
				s.createAuditLog(clusterID, "delete", "serviceaccount", resourceID, sa.Namespace, sa.Name)
				s.db.Delete(&sa)
			}
		}
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

		labelsJSON, _ := json.Marshal(labels)
		secretsJSON, _ := json.Marshal(secrets)

		sa := models.ServiceAccount{
			ClusterID: clusterID,
			UID:       uid,
			Name:      name,
			Namespace: namespace,
			Labels:    string(labelsJSON),
			Secrets:   string(secretsJSON),
		}

		// Check if SA exists in DB
		var existingSA models.ServiceAccount
		err := s.db.Where("cluster_id = ? AND uid = ?", clusterID, uid).First(&existingSA).Error

		if err == nil {
			// SA exists - check if it changed
			changed := existingSA.Name != sa.Name ||
				existingSA.Namespace != sa.Namespace ||
				existingSA.Labels != sa.Labels ||
				existingSA.Secrets != sa.Secrets

			if changed {
				// Update SA
				s.db.Model(&existingSA).Updates(map[string]interface{}{
					"name":      sa.Name,
					"namespace": sa.Namespace,
					"labels":    sa.Labels,
					"secrets":   sa.Secrets,
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

// GetAgentHandler returns gin handler for agent sync
func GetAgentHandler(db *gorm.DB) gin.HandlerFunc {
	service := NewAgentService(db)
	return func(c *gin.Context) {
		var req struct {
			ClusterID string                 `json:"clusterId"`
			Data      map[string]interface{} `json:"data"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body", "details": err.Error()})
			return
		}

		if req.ClusterID == "" {
			c.JSON(400, gin.H{"error": "clusterId is required"})
			return
		}

		if err := service.SyncData(req.ClusterID, req.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to sync data", "details": err.Error()})
			return
		}

		c.JSON(200, gin.H{"success": true, "message": "Data synced successfully"})
	}
}
