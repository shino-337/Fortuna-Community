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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/fortuna/core/pkg/securityaudit"
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

var safeRuleIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

func policyRuleUID(c *gin.Context) string {
	if uid := strings.TrimSpace(c.Param("uid")); uid != "" {
		return uid
	}
	return strings.TrimSpace(c.Param("id"))
}

func resolveRuleFile(rulesDir, ruleID string) (string, error) {
	ruleID = strings.TrimSpace(ruleID)
	if !safeRuleIDPattern.MatchString(ruleID) || strings.Contains(ruleID, "..") {
		return "", fmt.Errorf("invalid rule id")
	}
	absDir, err := filepath.Abs(rulesDir)
	if err != nil {
		return "", fmt.Errorf("resolve rules directory: %w", err)
	}
	target := filepath.Join(absDir, ruleID+".yaml")
	rel, err := filepath.Rel(absDir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("rule id escapes rules directory")
	}
	return target, nil
}

func chunkRuleIDs(ids []string, size int) [][]string {
	if size <= 0 {
		size = 200
	}
	var out [][]string
	for i := 0; i < len(ids); i += size {
		j := i + size
		if j > len(ids) {
			j = len(ids)
		}
		out = append(out, ids[i:j])
	}
	return out
}

func collectRuleActivationMeta(db *gorm.DB, rules []riskengine.Rule) map[string]ruleActivationMeta {
	meta := make(map[string]ruleActivationMeta, len(rules))
	if db == nil || len(rules) == 0 {
		return meta
	}

	ruleIDs := make([]string, 0, len(rules))
	for _, r := range rules {
		if id := strings.TrimSpace(r.ID); id != "" {
			ruleIDs = append(ruleIDs, id)
		}
	}
	if len(ruleIDs) == 0 {
		return meta
	}
	for _, id := range ruleIDs {
		meta[id] = ruleActivationMeta{RelatedCapabilities: []string{}}
	}

	t24 := time.Now().Add(-24 * time.Hour)
	t7 := time.Now().Add(-7 * 24 * time.Hour)

	type totalRow struct {
		CVEID string     `gorm:"column:cve_id"`
		Cnt   int64      `gorm:"column:cnt"`
		Last  *time.Time `gorm:"column:last_ts"`
	}
	type cntRow struct {
		CVEID string `gorm:"column:cve_id"`
		Cnt   int64  `gorm:"column:cnt"`
	}

	for _, chunk := range chunkRuleIDs(ruleIDs, 200) {
		var totals []totalRow
		if err := db.Model(&models.Insight{}).
			Select("cve_id AS cve_id, COUNT(*) AS cnt, MAX(created_at) AS last_ts").
			Scopes(scopeInsightsForYAMLRuleIDsIn(chunk)).
			Group("cve_id").
			Scan(&totals).Error; err != nil {
			log.Printf("[collectRuleActivationMeta] totals: %v", err)
		}
		for _, row := range totals {
			if m, ok := meta[row.CVEID]; ok {
				m.Count = row.Cnt
				m.LastSeenAt = row.Last
				meta[row.CVEID] = m
			}
		}

		var c24 []cntRow
		_ = db.Model(&models.Insight{}).
			Select("cve_id, COUNT(*) AS cnt").
			Scopes(scopeInsightsForYAMLRuleIDsIn(chunk)).
			Where("created_at >= ?", t24).
			Group("cve_id").
			Scan(&c24)
		for _, row := range c24 {
			if m, ok := meta[row.CVEID]; ok {
				m.ImpactedFindings24h = row.Cnt
				meta[row.CVEID] = m
			}
		}

		var c7 []cntRow
		_ = db.Model(&models.Insight{}).
			Select("cve_id, COUNT(*) AS cnt").
			Scopes(scopeInsightsForYAMLRuleIDsIn(chunk)).
			Where("created_at >= ?", t7).
			Group("cve_id").
			Scan(&c7)
		for _, row := range c7 {
			if m, ok := meta[row.CVEID]; ok {
				m.ImpactedFindings7d = row.Cnt
				meta[row.CVEID] = m
			}
		}
	}

	var recent []models.Insight
	_ = db.Model(&models.Insight{}).
		Select("cve_id", "evidence").
		Where("cve_id IN ?", ruleIDs).
		Order("created_at DESC").
		Limit(2500).
		Find(&recent).Error

	consumed := make(map[string]int, len(ruleIDs))
	capFreqByRule := make(map[string]map[string]int, len(ruleIDs))
	for _, id := range ruleIDs {
		capFreqByRule[id] = map[string]int{}
	}
	for _, ins := range recent {
		rid := strings.TrimSpace(ins.CVEID)
		if rid == "" {
			continue
		}
		if _, ok := meta[rid]; !ok {
			continue
		}
		if consumed[rid] >= 200 {
			continue
		}
		consumed[rid]++
		for _, cap := range extractCapabilityIDsFromEvidence(ins.Evidence) {
			capFreqByRule[rid][cap]++
		}
	}
	for rid, freq := range capFreqByRule {
		m := meta[rid]
		m.RelatedCapabilities = topNCapabilityKeys(freq, 3)
		meta[rid] = m
	}
	return meta
}

func scopeInsightsForYAMLRuleIDsIn(ids []string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(ids) == 0 {
			return db.Where("1 = 0")
		}
		return db.Where("cve_id IN ?", ids)
	}
}

// scopeInsightsForYAMLRuleID matches findings produced by YAML rules (cve_id holds rule.ID).
func scopeInsightsForYAMLRuleID(ruleID string) func(*gorm.DB) *gorm.DB {
	rid := strings.TrimSpace(ruleID)
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("cve_id = ?", rid)
	}
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

var (
	globalRulesManager *RulesManager
	rulesManagerOnce   sync.Once
)

// GetRulesManager returns the global rules manager (initialized once per process).
func GetRulesManager(db *gorm.DB) *RulesManager {
	rulesManagerOnce.Do(func() {
		rulesDir := os.Getenv("FORTUNA_RULES_DIR")
		if rulesDir == "" {
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
	})
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
				"uid":         rule.ID,
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
		ruleID := policyRuleUID(c)
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

		// Match count: YAML rules persist rule id in insights.cve_id (see riskengine.createInsight).
		var matchCount int64
		db.Model(&models.Insight{}).Scopes(scopeInsightsForYAMLRuleID(ruleID)).Count(&matchCount)

		var recentMatches []models.Insight
		db.Model(&models.Insight{}).Scopes(scopeInsightsForYAMLRuleID(ruleID)).
			Order("created_at DESC").
			Limit(10).
			Find(&recentMatches)

		source := collectRuleSource(db, manager)[ruleID]
		decoration := decorateRules(rules)[ruleID]
		activation := collectRuleActivationMeta(db, []riskengine.Rule{*rule})[ruleID]

		c.JSON(http.StatusOK, gin.H{
			"rule":                rule,
			"uid":                 ruleID,
			"ruleUid":             ruleID,
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
		filename, err := resolveRuleFile(manager.rulesDir, rule.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
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
		ruleID := policyRuleUID(c)
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

		filename, err := resolveRuleFile(manager.rulesDir, ruleID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

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
		ruleID := policyRuleUID(c)
		manager := GetRulesManager(db)

		if manager.rulesDir == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Rules directory not configured"})
			return
		}

		filename, err := resolveRuleFile(manager.rulesDir, ruleID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
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

// TestRule tests a rule against sample resource data using the full YAMLEngine
// (including CEL expression evaluation).
func TestRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := policyRuleUID(c)
		manager := GetRulesManager(db)
		if manager == nil || manager.yamlEngine == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "rules engine not available"})
			return
		}

		var request struct {
			Resource map[string]interface{} `json:"resource" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

		conditionResults := make([]gin.H, 0, len(rule.Conditions))
		overallMatched := true

		for i, condition := range rule.Conditions {
			entry := gin.H{
				"index": i,
				"type":  condition.Type,
			}

			switch condition.Type {
			case riskengine.CondTypeResource:
				entry["field"] = condition.Field
				entry["operator"] = condition.Operator
				entry["expectedValue"] = condition.Value
				fieldValue := getNestedField(request.Resource, condition.Field)
				entry["actualValue"] = fieldValue
				if fieldValue != nil {
					entry["matched"] = true
				} else {
					entry["matched"] = false
					overallMatched = false
				}

			case riskengine.CondTypeExpression:
				entry["expression"] = condition.Expression
				celResult, celErr := manager.yamlEngine.TestCELCondition(condition, request.Resource)
				if celErr != nil {
					entry["matched"] = false
					entry["error"] = celErr.Error()
					overallMatched = false
				} else {
					entry["matched"] = celResult
					if !celResult {
						overallMatched = false
					}
				}

			default:
				entry["matched"] = false
				entry["error"] = fmt.Sprintf("unknown condition type: %s", condition.Type)
				overallMatched = false
			}

			conditionResults = append(conditionResults, entry)
		}

		if rule.Aggregation == riskengine.AggregationOR {
			overallMatched = false
			for _, cr := range conditionResults {
				if m, ok := cr["matched"].(bool); ok && m {
					overallMatched = true
					break
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"match":       overallMatched,
			"rule":        rule,
			"aggregation": rule.Aggregation,
			"conditions":  conditionResults,
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
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			"policy_rules_reload",
			"policy",
			"yaml_engine",
			"success",
			"high",
			"jwt",
			nil,
			map[string]any{"rule_count": len(rules), "duration": time.Since(start).String()},
			nil,
			nil,
		)
		securityaudit.Append(db, &ev)
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
		ruleID := policyRuleUID(c)

		var matchCount int64
		db.Model(&models.Insight{}).Scopes(scopeInsightsForYAMLRuleID(ruleID)).Count(&matchCount)

		var recentMatches []models.Insight
		db.Model(&models.Insight{}).Scopes(scopeInsightsForYAMLRuleID(ruleID)).
			Order("created_at DESC").
			Limit(20).
			Find(&recentMatches)

		c.JSON(http.StatusOK, gin.H{
			"ruleId":        ruleID,
			"uid":           ruleID,
			"ruleUid":       ruleID,
			"totalMatches":  matchCount,
			"recentMatches": recentMatches,
		})
	}
}

// GetRuleMatches returns recent matches for a rule
func GetRuleMatches(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := policyRuleUID(c)
		limitStr := c.DefaultQuery("limit", "50")
		limit, _ := strconv.Atoi(limitStr)

		if limit > 100 {
			limit = 100
		}

		var matches []models.Insight
		db.Model(&models.Insight{}).Scopes(scopeInsightsForYAMLRuleID(ruleID)).
			Order("created_at DESC").
			Limit(limit).
			Find(&matches)

		c.JSON(http.StatusOK, gin.H{
			"ruleId":  ruleID,
			"uid":     ruleID,
			"ruleUid": ruleID,
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
