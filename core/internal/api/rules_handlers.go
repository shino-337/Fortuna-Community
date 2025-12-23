package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
)

// RulesManager manages rules engine instance
type RulesManager struct {
	yamlEngine *riskengine.YAMLEngine
	rulesDir   string
}

var globalRulesManager *RulesManager

// GetRulesManager returns the global rules manager
func GetRulesManager(db *gorm.DB) *RulesManager {
	if globalRulesManager == nil {
		rulesDir := os.Getenv("KSAM_RULES_DIR")
		if rulesDir == "" {
			// Try default paths
			if _, err := os.Stat("./rules"); err == nil {
				rulesDir = "./rules"
			} else if _, err := os.Stat("core/rules"); err == nil {
				rulesDir = "core/rules"
			}
		}

		var yamlEngine *riskengine.YAMLEngine
		if rulesDir != "" {
			if ye, err := riskengine.NewYAMLEngine(db, rulesDir); err == nil {
				yamlEngine = ye
			} else {
				log.Printf("[RulesManager] Failed to create YAML engine: %v", err)
			}
		}

		globalRulesManager = &RulesManager{
			yamlEngine: yamlEngine,
			rulesDir:   rulesDir,
		}
	}
	return globalRulesManager
}

// GetRules returns all rules
func GetRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		manager := GetRulesManager(db)
		
		var rules []riskengine.Rule
		if manager.yamlEngine != nil {
			rules = manager.yamlEngine.GetRules()
		} else {
			// Fallback to hardcoded rules
			rules = riskengine.GetBuiltInRules()
		}

		// Filter by status
		status := c.Query("status")
		filteredRules := []riskengine.Rule{}
		for _, rule := range rules {
			if status == "" || (status == "active" && rule.Enabled) || (status == "disabled" && !rule.Enabled) {
				filteredRules = append(filteredRules, rule)
			}
		}

		// Filter by category
		category := c.Query("category")
		if category != "" {
			filtered := []riskengine.Rule{}
			for _, rule := range filteredRules {
				if string(rule.Category) == category {
					filtered = append(filtered, rule)
				}
			}
			filteredRules = filtered
		}

		// Filter by severity
		severity := c.Query("severity")
		if severity != "" {
			filtered := []riskengine.Rule{}
			for _, rule := range filteredRules {
				if string(rule.Severity) == severity {
					filtered = append(filtered, rule)
				}
			}
			filteredRules = filtered
		}

		// Count active/disabled
		activeCount := 0
		disabledCount := 0
		for _, rule := range rules {
			if rule.Enabled {
				activeCount++
			} else {
				disabledCount++
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"rules":        filteredRules,
			"total":        len(filteredRules),
			"active":       activeCount,
			"disabled":     disabledCount,
			"totalAll":     len(rules),
		})
	}
}

// GetRule returns a specific rule by ID
func GetRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("id")
		manager := GetRulesManager(db)

		var rule *riskengine.Rule
		if manager.yamlEngine != nil {
			rules := manager.yamlEngine.GetRules()
			for _, r := range rules {
				if r.ID == ruleID {
					rule = &r
					break
				}
			}
		} else {
			// Fallback to hardcoded rules
			rules := riskengine.GetBuiltInRules()
			for _, r := range rules {
				if r.ID == ruleID {
					rule = &r
					break
				}
			}
		}

		if rule == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
			return
		}

		// Get rule metrics (match count from insights)
		var matchCount int64
		db.Model(&models.Insight{}).Where("type LIKE ?", "%"+ruleID+"%").Count(&matchCount)

		// Get recent matches
		var recentMatches []models.Insight
		db.Where("type LIKE ?", "%"+ruleID+"%").
			Order("created_at DESC").
			Limit(10).
			Find(&recentMatches)

		c.JSON(http.StatusOK, gin.H{
			"rule":         rule,
			"matchCount":   matchCount,
			"recentMatches": recentMatches,
		})
	}
}

// CreateRule creates a new rule from YAML
func CreateRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			YAML    string `json:"yaml" binding:"required"`
			Enabled bool   `json:"enabled"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		manager := GetRulesManager(db)
		if manager.rulesDir == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rules directory not configured"})
			return
		}

		// Parse YAML
		var rule riskengine.Rule
		if err := yaml.Unmarshal([]byte(request.YAML), &rule); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid YAML: %v", err)})
			return
		}

		// Validate rule
		if rule.ID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rule ID is required"})
			return
		}

		// Set enabled status
		rule.Enabled = request.Enabled

		// Save to file
		filename := filepath.Join(manager.rulesDir, rule.ID+".yaml")
		if err := os.WriteFile(filename, []byte(request.YAML), 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save rule: %v", err)})
			return
		}

		// Reload rules
		if manager.yamlEngine != nil {
			if err := manager.yamlEngine.Reload(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload rules: %v", err)})
				return
			}
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "Rule created successfully",
			"rule":    rule,
		})
	}
}

// UpdateRule updates an existing rule
func UpdateRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("id")
		manager := GetRulesManager(db)

		var request struct {
			Enabled *bool   `json:"enabled"`
			YAML    *string `json:"yaml"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if manager.rulesDir == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rules directory not configured"})
			return
		}

		filename := filepath.Join(manager.rulesDir, ruleID+".yaml")
		
		// If YAML provided, update file
		if request.YAML != nil {
			if err := os.WriteFile(filename, []byte(*request.YAML), 0644); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update rule: %v", err)})
				return
			}
		} else if request.Enabled != nil {
			// Just update enabled status in existing file
			data, err := os.ReadFile(filename)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Rule file not found"})
				return
			}

			var rule riskengine.Rule
			if err := yaml.Unmarshal(data, &rule); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid YAML: %v", err)})
				return
			}

			rule.Enabled = *request.Enabled
			updatedYAML, err := yaml.Marshal(&rule)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to marshal rule: %v", err)})
				return
			}

			if err := os.WriteFile(filename, updatedYAML, 0644); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save rule: %v", err)})
				return
			}
		}

		// Reload rules
		if manager.yamlEngine != nil {
			if err := manager.yamlEngine.Reload(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload rules: %v", err)})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Rule updated successfully",
		})
	}
}

// DeleteRule deletes a rule
func DeleteRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("id")
		manager := GetRulesManager(db)

		if manager.rulesDir == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rules directory not configured"})
			return
		}

		filename := filepath.Join(manager.rulesDir, ruleID+".yaml")
		if err := os.Remove(filename); err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to delete rule: %v", err)})
			return
		}

		// Reload rules
		if manager.yamlEngine != nil {
			if err := manager.yamlEngine.Reload(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to reload rules: %v", err)})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Rule deleted successfully",
		})
	}
}

// TestRule tests a rule against sample data
func TestRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("id")
		manager := GetRulesManager(db)

		var request struct {
			Resource map[string]interface{} `json:"resource" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Find rule
		var rule *riskengine.Rule
		if manager.yamlEngine != nil {
			rules := manager.yamlEngine.GetRules()
			for _, r := range rules {
				if r.ID == ruleID {
					rule = &r
					break
				}
			}
		} else {
			rules := riskengine.GetBuiltInRules()
			for _, r := range rules {
				if r.ID == ruleID {
					rule = &r
					break
				}
			}
		}

		if rule == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Rule not found"})
			return
		}

		// Convert resource to JSON for evaluation
		resourceJSON, _ := json.Marshal(request.Resource)
		_ = string(resourceJSON) // Reserved for future use

		// Simple evaluation (check if rule would match)
		// This is a simplified version - full evaluation would use the risk engine
		matched := false
		details := map[string]interface{}{}

		// Check conditions
		for _, condition := range rule.Conditions {
			if condition.Type == riskengine.CondTypeResource {
				// Simple field check
				fieldValue := getNestedField(request.Resource, condition.Field)
				if fieldValue != nil {
					matched = true
					details[condition.Field] = fieldValue
				}
			} else if condition.Type == riskengine.CondTypeExpression {
				// For CEL expressions, we'd need to compile and evaluate
				// For now, just indicate that expression exists
				details["expression"] = condition.Expression
				matched = true // Simplified
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"match":   matched,
			"rule":    rule,
			"details": details,
		})
	}
}

// ReloadRules reloads all rules
func ReloadRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		manager := GetRulesManager(db)

		if manager.yamlEngine == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "YAML engine not available"})
			return
		}

		start := time.Now()
		if err := manager.yamlEngine.Reload(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   err.Error(),
				"reloaded": false,
			})
			return
		}

		rules := manager.yamlEngine.GetRules()
		c.JSON(http.StatusOK, gin.H{
			"message":   "Rules reloaded successfully",
			"reloaded":  true,
			"count":     len(rules),
			"duration": time.Since(start).String(),
		})
	}
}

// GetRuleMetrics returns performance metrics for a rule
func GetRuleMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("id")

		// Get match count from insights
		var matchCount int64
		db.Model(&models.Insight{}).Where("type LIKE ? OR description LIKE ?", "%"+ruleID+"%", "%"+ruleID+"%").Count(&matchCount)

		// Get recent matches
		var recentMatches []models.Insight
		db.Where("type LIKE ? OR description LIKE ?", "%"+ruleID+"%", "%"+ruleID+"%").
			Order("created_at DESC").
			Limit(20).
			Find(&recentMatches)

		c.JSON(http.StatusOK, gin.H{
			"ruleId":        ruleID,
			"totalMatches":  matchCount,
			"recentMatches": recentMatches,
		})
	}
}

// GetRuleMatches returns recent matches for a rule
func GetRuleMatches(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("id")
		limitStr := c.DefaultQuery("limit", "50")
		limit, _ := strconv.Atoi(limitStr)

		if limit > 100 {
			limit = 100
		}

		var matches []models.Insight
		db.Where("type LIKE ? OR description LIKE ?", "%"+ruleID+"%", "%"+ruleID+"%").
			Order("created_at DESC").
			Limit(limit).
			Find(&matches)

		c.JSON(http.StatusOK, gin.H{
			"ruleId": ruleID,
			"matches": matches,
			"count":   len(matches),
		})
	}
}

// Helper function to get nested field from map
func getNestedField(obj map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := obj

	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	return nil
}

