package api

import (
	"crypto/sha1"
	"encoding/hex"
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

type ruleActivationMeta struct {
	Count               int64
	LastSeenAt          *time.Time
	ImpactedFindings24h int64
	ImpactedFindings7d  int64
	RelatedCapabilities []string
}

type ruleDecoration struct {
	Signature       string
	OverlapGroup    string
	IsCanonical     bool
	CanonicalRuleID string
}

func collectRuleActivationMeta(db *gorm.DB, rules []riskengine.Rule) map[string]ruleActivationMeta {
	meta := make(map[string]ruleActivationMeta, len(rules))
	if db == nil || len(rules) == 0 {
		return meta
	}

	for _, rule := range rules {
		ruleID := strings.TrimSpace(rule.ID)
		if ruleID == "" {
			continue
		}
		like := "%" + ruleID + "%"
		var row struct {
			Count int64
			Last  *time.Time
		}
		if err := db.Model(&models.Insight{}).
			Select("COUNT(*) as count, MAX(created_at) as last").
			Where("type LIKE ? OR description LIKE ?", like, like).
			Scan(&row).Error; err != nil {
			continue
		}
		meta[ruleID] = ruleActivationMeta{
			Count:      row.Count,
			LastSeenAt: row.Last,
		}

		var c24h int64
		_ = db.Model(&models.Insight{}).
			Where("(type LIKE ? OR description LIKE ?) AND created_at >= ?", like, like, time.Now().Add(-24*time.Hour)).
			Count(&c24h).Error
		var c7d int64
		_ = db.Model(&models.Insight{}).
			Where("(type LIKE ? OR description LIKE ?) AND created_at >= ?", like, like, time.Now().Add(-7*24*time.Hour)).
			Count(&c7d).Error

		var recent []models.Insight
		_ = db.Select("evidence").
			Where("type LIKE ? OR description LIKE ?", like, like).
			Order("created_at DESC").
			Limit(200).
			Find(&recent).Error

		capFreq := map[string]int{}
		for _, ins := range recent {
			for _, cap := range extractCapabilityIDsFromEvidence(ins.Evidence) {
				capFreq[cap]++
			}
		}
		topCaps := topNCapabilityKeys(capFreq, 3)
		cur := meta[ruleID]
		cur.ImpactedFindings24h = c24h
		cur.ImpactedFindings7d = c7d
		cur.RelatedCapabilities = topCaps
		meta[ruleID] = cur
	}
	return meta
}

func extractCapabilityIDsFromEvidence(evidence string) []string {
	if strings.TrimSpace(evidence) == "" {
		return nil
	}
	var payload interface{}
	if err := json.Unmarshal([]byte(evidence), &payload); err != nil {
		return nil
	}
	out := map[string]struct{}{}
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch t := v.(type) {
		case map[string]interface{}:
			for k, vv := range t {
				lk := strings.ToLower(strings.TrimSpace(k))
				if lk == "capabilityid" || lk == "capability_id" || lk == "capability" {
					if s, ok := vv.(string); ok && strings.TrimSpace(s) != "" {
						out[strings.TrimSpace(s)] = struct{}{}
					}
				}
				walk(vv)
			}
		case []interface{}:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(payload)
	res := make([]string, 0, len(out))
	for k := range out {
		res = append(res, k)
	}
	return res
}

func topNCapabilityKeys(freq map[string]int, n int) []string {
	type kv struct {
		Key string
		Val int
	}
	items := make([]kv, 0, len(freq))
	for k, v := range freq {
		items = append(items, kv{Key: k, Val: v})
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Val > items[i].Val || (items[j].Val == items[i].Val && items[j].Key < items[i].Key) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	if len(items) > n {
		items = items[:n]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Key)
	}
	return out
}

func collectRuleSource(db *gorm.DB, manager *RulesManager) map[string]string {
	out := map[string]string{}
	for _, r := range riskengine.GetBuiltInRules() {
		out[r.ID] = "built-in"
	}
	if manager != nil && manager.yamlEngine != nil {
		for id := range manager.yamlEngine.GetYAMLRuleIDs() {
			out[id] = "files"
		}
	}
	if db != nil && db.Migrator().HasTable(&models.RiskRule{}) {
		var ids []string
		if err := db.Model(&models.RiskRule{}).Where("deleted_at IS NULL").Pluck("rule_id", &ids).Error; err == nil {
			for _, id := range ids {
				out[id] = "db"
			}
		}
	}
	return out
}

func buildRuleSignature(rule riskengine.Rule) string {
	conditionsJSON, _ := json.Marshal(rule.Conditions)
	base := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%s|%s|%s|%s", rule.Category, rule.Severity, rule.Aggregation, string(conditionsJSON))))
	sum := sha1.Sum([]byte(base))
	return hex.EncodeToString(sum[:])[:12]
}

func decorateRules(rules []riskengine.Rule) map[string]ruleDecoration {
	out := make(map[string]ruleDecoration, len(rules))
	groupBySig := map[string][]riskengine.Rule{}
	for _, rule := range rules {
		sig := buildRuleSignature(rule)
		groupBySig[sig] = append(groupBySig[sig], rule)
	}
	for sig, group := range groupBySig {
		canonical := group[0]
		for _, g := range group[1:] {
			if g.BaseScore > canonical.BaseScore || (g.BaseScore == canonical.BaseScore && g.ID < canonical.ID) {
				canonical = g
			}
		}
		for _, rule := range group {
			out[rule.ID] = ruleDecoration{
				Signature:       sig,
				OverlapGroup:    "sig-" + sig,
				IsCanonical:     rule.ID == canonical.ID,
				CanonicalRuleID: canonical.ID,
			}
		}
	}
	return out
}

// RulesManager manages rules engine instance
type RulesManager struct {
	yamlEngine *riskengine.YAMLEngine
	rulesDir   string
}

var globalRulesManager *RulesManager

// GetRulesManager returns the global rules manager
func GetRulesManager(db *gorm.DB) *RulesManager {
	if globalRulesManager == nil {
		rulesDir := os.Getenv("FORTUNA_RULES_DIR")
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
		if manager.yamlEngine == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "rules engine not initialized from YAML directory",
			})
			return
		}

		rules := manager.yamlEngine.GetRules()

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
		activationMeta := collectRuleActivationMeta(db, filteredRules)
		ruleSource := collectRuleSource(db, manager)
		decorations := decorateRules(filteredRules)

		respRules := make([]gin.H, 0, len(filteredRules))
		for _, rule := range filteredRules {
			item := gin.H{
				"id":          rule.ID,
				"name":        rule.Name,
				"category":    rule.Category,
				"severity":    rule.Severity,
				"description": rule.Description,
				"enabled":     rule.Enabled,
				"conditions":  rule.Conditions,
				"aggregation": rule.Aggregation,
				"baseScore":   rule.BaseScore,
				"tags":        rule.Tags,
				"source":      ruleSource[rule.ID],
			}
			if d, ok := decorations[rule.ID]; ok {
				item["signature"] = d.Signature
				item["overlapGroup"] = d.OverlapGroup
				item["isCanonical"] = d.IsCanonical
				item["canonicalRuleId"] = d.CanonicalRuleID
			}
			if m, ok := activationMeta[rule.ID]; ok {
				item["matches"] = m.Count
				if m.LastSeenAt != nil {
					item["lastMatchedAt"] = m.LastSeenAt.UTC().Format(time.RFC3339)
				}
				item["impactedFindings24h"] = m.ImpactedFindings24h
				item["impactedFindings7d"] = m.ImpactedFindings7d
				item["relatedCapabilities"] = m.RelatedCapabilities
			} else {
				item["matches"] = int64(0)
				item["impactedFindings24h"] = int64(0)
				item["impactedFindings7d"] = int64(0)
				item["relatedCapabilities"] = []string{}
			}
			respRules = append(respRules, item)
		}

		c.JSON(http.StatusOK, gin.H{
			"rules":    respRules,
			"total":    len(filteredRules),
			"active":   activeCount,
			"disabled": disabledCount,
			"totalAll": len(rules),
		})
	}
}

// GetRule returns a specific rule by ID
func GetRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("id")
		manager := GetRulesManager(db)
		if manager.yamlEngine == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "rules engine not initialized from YAML directory",
			})
			return
		}

		var rule *riskengine.Rule
		rules := manager.yamlEngine.GetRules()
		for _, r := range rules {
			if r.ID == ruleID {
				rule = &r
				break
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

		source := collectRuleSource(db, manager)[ruleID]
		decoration := decorateRules(rules)[ruleID]
		activation := collectRuleActivationMeta(db, []riskengine.Rule{*rule})[ruleID]

		c.JSON(http.StatusOK, gin.H{
			"rule":                rule,
			"source":              source,
			"signature":           decoration.Signature,
			"overlapGroup":        decoration.OverlapGroup,
			"isCanonical":         decoration.IsCanonical,
			"canonicalRule":       decoration.CanonicalRuleID,
			"matchCount":          matchCount,
			"impactedFindings24h": activation.ImpactedFindings24h,
			"impactedFindings7d":  activation.ImpactedFindings7d,
			"relatedCapabilities": activation.RelatedCapabilities,
			"recentMatches":       recentMatches,
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
		if manager.yamlEngine == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "rules engine not initialized from YAML directory",
			})
			return
		}

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
		rules := manager.yamlEngine.GetRules()
		for _, r := range rules {
			if r.ID == ruleID {
				rule = &r
				break
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
				"error":    err.Error(),
				"reloaded": false,
			})
			return
		}

		rules := manager.yamlEngine.GetRules()
		c.JSON(http.StatusOK, gin.H{
			"message":  "Rules reloaded successfully",
			"reloaded": true,
			"count":    len(rules),
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
			"ruleId":  ruleID,
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
