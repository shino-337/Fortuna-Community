package riskengine

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ksam/core/pkg/models"
	"gorm.io/gorm"
)

// InsightManager manages insight creation and updates
type InsightManager struct {
	db *gorm.DB
}

// NewInsightManager creates a new insight manager
func NewInsightManager(db *gorm.DB) *InsightManager {
	return &InsightManager{db: db}
}

// CreateOrUpdateInsight creates or updates an insight with improved duplicate detection
func (m *InsightManager) CreateOrUpdateInsight(insight *models.Insight) error {
	// Parse affected resources to build unique key
	var affectedResources []map[string]interface{}
	if err := json.Unmarshal([]byte(insight.AffectedResources), &affectedResources); err != nil {
		// If parsing fails, create new insight
		return m.createInsight(insight)
	}

	if len(affectedResources) == 0 {
		return fmt.Errorf("no affected resources")
	}

	// Build unique key from affected resources
	// Use a more robust approach: hash of type + first resource identifier
	firstResource := affectedResources[0]
	resourceType, _ := firstResource["type"].(string)
	resourceName, _ := firstResource["name"].(string)
	resourceNamespace, _ := firstResource["namespace"].(string)

	// Build unique identifier: type + resource type + name + namespace
	// This ensures we don't create duplicates for the same resource and risk type
	var existing models.Insight

	// Try to find existing insight using a more precise match
	// Match by: type, severity, and affected resources containing the same resource
	query := m.db.Where("type = ? AND severity = ?", insight.Type, insight.Severity)

	// Match by resource identifier in affected resources JSON
	resourceIdentifier := fmt.Sprintf(`"name":"%s"`, resourceName)
	if resourceNamespace != "" {
		resourceIdentifier = fmt.Sprintf(`"name":"%s","namespace":"%s"`, resourceName, resourceNamespace)
	}

	query = query.Where("affected_resources::text LIKE ?", fmt.Sprintf("%% %s %%", resourceIdentifier))

	result := query.First(&existing)

	if result.Error == nil {
		// Update existing insight (only if description or affected resources changed)
		needsUpdate := false
		if existing.Description != insight.Description {
			existing.Description = insight.Description
			needsUpdate = true
		}
		if existing.AffectedResources != insight.AffectedResources {
			existing.AffectedResources = insight.AffectedResources
			needsUpdate = true
		}
		if existing.RecommendedAction != insight.RecommendedAction {
			existing.RecommendedAction = insight.RecommendedAction
			needsUpdate = true
		}

		if needsUpdate {
			existing.UpdatedAt = time.Now()
			if err := m.db.Save(&existing).Error; err != nil {
				return fmt.Errorf("failed to update insight: %w", err)
			}
			log.Printf("[InsightManager] Updated insight ID=%d for %s/%s/%s",
				existing.ID, resourceType, resourceNamespace, resourceName)
		} else {
			log.Printf("[InsightManager] Insight already exists (ID=%d), no update needed", existing.ID)
		}
		return nil
	} else if result.Error == gorm.ErrRecordNotFound {
		// Create new insight
		return m.createInsight(insight)
	} else {
		return fmt.Errorf("failed to check existing insight: %w", result.Error)
	}
}

// createInsight creates a new insight
func (m *InsightManager) createInsight(insight *models.Insight) error {
	if err := m.db.Create(insight).Error; err != nil {
		return fmt.Errorf("failed to create insight: %w", err)
	}

	log.Printf("[InsightManager] Created insight ID=%d: %s", insight.ID, insight.Description)
	return nil
}

// GetInsightsByType returns insights by type
func (m *InsightManager) GetInsightsByType(insightType string) ([]models.Insight, error) {
	var insights []models.Insight
	if err := m.db.Where("type = ?", insightType).Find(&insights).Error; err != nil {
		return nil, fmt.Errorf("failed to get insights: %w", err)
	}
	return insights, nil
}

// GetInsightsBySeverity returns insights by severity
func (m *InsightManager) GetInsightsBySeverity(severity string) ([]models.Insight, error) {
	var insights []models.Insight
	if err := m.db.Where("severity = ?", severity).Find(&insights).Error; err != nil {
		return nil, fmt.Errorf("failed to get insights: %w", err)
	}
	return insights, nil
}
