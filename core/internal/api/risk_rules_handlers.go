package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gopkg.in/yaml.v3"
)

// flexibleString unmarshals JSON number or string into string (avoids "cannot unmarshal number into Go struct field Rule.id of type string" when client sends id as number).
type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case string:
		*s = flexibleString(x)
	case float64:
		*s = flexibleString(strconv.FormatInt(int64(x), 10))
	case nil:
		*s = ""
	default:
		return fmt.Errorf("id: expected string or number, got %T", v)
	}
	return nil
}

// GetRiskRulesList returns risk rules: from DB (risk_rules table) when present, else from FORTUNA_RULES_DIR (read-only).
// GET /api/v1/risk-rules
func GetRiskRulesList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db != nil && db.Migrator().HasTable(&models.RiskRule{}) {
			var rows []models.RiskRule
			if err := db.Where("deleted_at IS NULL").Order("rule_id").Find(&rows).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			list := make([]riskengine.RiskRuleSummary, 0, len(rows))
			for i := range rows {
				list = append(list, riskengine.RiskRuleSummary{
					ID:          rows[i].RuleID,
					Name:        rows[i].Name,
					Severity:    rows[i].Severity,
					Description: rows[i].Description,
					Category:    rows[i].Category,
					File:        "",
					Enabled:     rows[i].Enabled,
				})
			}
			log.Printf("[RiskRules] GET /risk/rules: returning %d rules from db", len(list))
			c.JSON(http.StatusOK, gin.H{"rules": list, "total": len(list), "source": "db"})
			return
		}
		rulesDir := os.Getenv("FORTUNA_RULES_DIR")
		list, err := riskengine.ListRuleSummariesFromDir(rulesDir)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if list == nil {
			list = []riskengine.RiskRuleSummary{}
		}
		log.Printf("[RiskRules] GET /risk/rules: returning %d rules from files (risk_rules table missing or not used)", len(list))
		c.JSON(http.StatusOK, gin.H{"rules": list, "total": len(list), "source": "files"})
	}
}

// GetRiskRuleByID returns a single risk rule by rule_id (from DB).
// GET /api/v1/risk-rules/:id
func GetRiskRuleByID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
			return
		}
		var row models.RiskRule
		if err := db.Where("rule_id = ? AND deleted_at IS NULL", id).First(&row).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp, err := riskRuleToAPI(&row)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// isYAMLContentType returns true if Content-Type indicates YAML body.
func isYAMLContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	return strings.Contains(ct, "application/x-yaml") ||
		strings.Contains(ct, "text/yaml") ||
		strings.Contains(ct, "application/yaml")
}

// bindRuleFromBody parses request body as JSON or YAML into a Rule. Content-Type decides format.
func bindRuleFromBody(c *gin.Context) (*riskengine.Rule, error) {
	body, err := c.GetRawData()
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}
	ct := c.GetHeader("Content-Type")
	var req riskengine.Rule
	if isYAMLContentType(ct) {
		if err := yaml.Unmarshal(body, &req); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
	} else {
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, fmt.Errorf("invalid JSON: %w", err)
		}
	}
	return &req, nil
}

// ValidateRiskRule validates a rule payload without saving. POST /api/v1/risk/rules/validate
// Accepts JSON or YAML (Content-Type: application/x-yaml or text/yaml). All validation server-side.
func ValidateRiskRule() gin.HandlerFunc {
	return func(c *gin.Context) {
		req, err := bindRuleFromBody(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"valid": false, "errors": []string{err.Error()}})
			return
		}
		errs := riskengine.ValidateRule(req)
		if len(errs) > 0 {
			c.JSON(http.StatusOK, gin.H{"valid": false, "errors": errs})
			return
		}
		c.JSON(http.StatusOK, gin.H{"valid": true, "errors": []string{}})
	}
}

// ImportRiskRule creates a rule from YAML or JSON body. POST /api/v1/risk/rules/import
// Verify first via POST /risk/rules/validate. Validation is enforced server-side.
func ImportRiskRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		req, err := bindRuleFromBody(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errs := riskengine.ValidateRule(req); len(errs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed", "errors": errs})
			return
		}
		m, err := riskengine.RuleToRiskRule(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := db.Create(m).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_ = riskengine.ReloadGlobalFromDB()
		if dir := riskengine.GetRiskRulesExportDir(); dir != "" {
			_ = riskengine.ExportRuleToFile(req, dir)
		}
		resp, _ := riskRuleToAPI(m)
		c.JSON(http.StatusCreated, resp)
	}
}

// ExportRiskRulesYAML returns one or all risk rules as YAML. GET /api/v1/risk/rules/export?id=xxx or no id for all.
func ExportRiskRulesYAML(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil || !db.Migrator().HasTable(&models.RiskRule{}) {
			c.Data(http.StatusOK, "application/x-yaml", []byte("# no rules\n"))
			return
		}
		id := c.Query("id")
		var rows []models.RiskRule
		if id != "" {
			var one models.RiskRule
			if err := db.Where("rule_id = ? AND deleted_at IS NULL", id).First(&one).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			rows = []models.RiskRule{one}
		} else {
			if err := db.Where("deleted_at IS NULL").Order("rule_id").Find(&rows).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		rules := make([]riskengine.Rule, 0, len(rows))
		for i := range rows {
			r, err := riskRuleToRule(&rows[i])
			if err != nil {
				continue
			}
			rules = append(rules, *r)
		}
		var out []byte
		if len(rules) == 1 {
			out, _ = yaml.Marshal(&rules[0])
		} else {
			out, _ = yaml.Marshal(rules)
		}
		c.Header("Content-Disposition", "attachment; filename=risk-rules.yaml")
		c.Data(http.StatusOK, "application/x-yaml", out)
	}
}

// riskRuleToRule converts DB model to riskengine.Rule (for export).
func riskRuleToRule(m *models.RiskRule) (*riskengine.Rule, error) {
	var conditions []riskengine.Condition
	if m.Conditions != "" {
		if err := json.Unmarshal([]byte(m.Conditions), &conditions); err != nil {
			return nil, err
		}
	}
	var tags []string
	if m.Tags != "" {
		_ = json.Unmarshal([]byte(m.Tags), &tags)
	}
	return &riskengine.Rule{
		ID:          m.RuleID,
		Name:        m.Name,
		Category:    riskengine.RuleCategory(m.Category),
		Severity:    riskengine.Severity(m.Severity),
		Description: m.Description,
		Enabled:     m.Enabled,
		Conditions:  conditions,
		Aggregation: riskengine.AggregationType(m.Aggregation),
		BaseScore:   m.BaseScore,
		Tags:        tags,
	}, nil
}

// CreateRiskRule creates a risk rule in DB. Validation is enforced server-side only; never trust client.
// POST /api/v1/risk-rules
func CreateRiskRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req riskengine.Rule
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Server-side validation (required for security; client can be bypassed)
		if errs := riskengine.ValidateRule(&req); len(errs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed", "errors": errs})
			return
		}
		m, err := riskengine.RuleToRiskRule(&req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := db.Create(m).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_ = riskengine.ReloadGlobalFromDB()
		if dir := riskengine.GetRiskRulesExportDir(); dir != "" {
			_ = riskengine.ExportRuleToFile(&req, dir)
		}
		resp, _ := riskRuleToAPI(m)
		c.JSON(http.StatusCreated, resp)
	}
}

// updateRiskRuleRequest mirrors riskengine.Rule but ID accepts JSON number or string (API returns id as number; client may send it back).
type updateRiskRuleRequest struct {
	ID          flexibleString           `json:"id"`
	Name        string                  `json:"name"`
	Category    riskengine.RuleCategory  `json:"category"`
	Severity    riskengine.Severity      `json:"severity"`
	Description string                  `json:"description"`
	Enabled     bool                    `json:"enabled"`
	Conditions  []riskengine.Condition   `json:"conditions"`
	Aggregation riskengine.AggregationType `json:"aggregation"`
	BaseScore   float64                 `json:"base_score"`
	Tags        []string                `json:"tags,omitempty"`
}

// UpdateRiskRule updates a risk rule by rule_id.
// PUT /api/v1/risk-rules/:id
func UpdateRiskRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
			return
		}
		var req updateRiskRuleRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		rule := riskengine.Rule{
			ID:          id,
			Name:        req.Name,
			Category:    req.Category,
			Severity:    req.Severity,
			Description: req.Description,
			Enabled:     req.Enabled,
			Conditions:  req.Conditions,
			Aggregation: req.Aggregation,
			BaseScore:   req.BaseScore,
			Tags:        req.Tags,
		}
		// Server-side validation (required for security; client can be bypassed)
		if errs := riskengine.ValidateRule(&rule); len(errs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed", "errors": errs})
			return
		}
		m, err := riskengine.RuleToRiskRule(&rule)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var row models.RiskRule
		if err := db.Where("rule_id = ? AND deleted_at IS NULL", id).First(&row).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		m.ID = row.ID
		m.CreatedAt = row.CreatedAt
		// Phase 3 rule versioning: snapshot current state to risk_rules_history before update
		if db.Migrator().HasTable(&models.RiskRuleHistory{}) {
			snapshotBytes, _ := json.Marshal(row)
			var nextVersion int
			db.Model(&models.RiskRuleHistory{}).Where("rule_id = ?", id).Select("COALESCE(MAX(version),0)+1").Scan(&nextVersion)
			hist := models.RiskRuleHistory{
				RuleID:    id,
				Version:   nextVersion,
				Snapshot:  string(snapshotBytes),
				CreatedAt: time.Now(),
			}
			_ = db.Create(&hist).Error
		}
		if err := db.Save(m).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_ = riskengine.ReloadGlobalFromDB()
		if dir := riskengine.GetRiskRulesExportDir(); dir != "" {
			_ = riskengine.ExportRuleToFile(&rule, dir)
		}
		resp, _ := riskRuleToAPI(m)
		c.JSON(http.StatusOK, resp)
	}
}

// DeleteRiskRule soft-deletes a risk rule by rule_id.
// DELETE /api/v1/risk-rules/:id
func DeleteRiskRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
			return
		}
		result := db.Where("rule_id = ?", id).Delete(&models.RiskRule{})
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		_ = riskengine.ReloadGlobalFromDB()
		if dir := riskengine.GetRiskRulesExportDir(); dir != "" {
			_ = riskengine.RemoveRuleFile(dir, id)
		}
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	}
}

// riskRuleToAPI converts DB model to API response (full rule for get/create/update).
func riskRuleToAPI(m *models.RiskRule) (gin.H, error) {
	var conditions []riskengine.Condition
	if m.Conditions != "" {
		_ = json.Unmarshal([]byte(m.Conditions), &conditions)
	}
	var tags []string
	if m.Tags != "" {
		_ = json.Unmarshal([]byte(m.Tags), &tags)
	}
	return gin.H{
		"id":          m.ID,
		"ruleId":      m.RuleID,
		"name":        m.Name,
		"category":    m.Category,
		"severity":    m.Severity,
		"description": m.Description,
		"enabled":     m.Enabled,
		"conditions":  conditions,
		"aggregation": m.Aggregation,
		"base_score":  m.BaseScore,
		"tags":        tags,
		"createdAt":   m.CreatedAt,
		"updatedAt":   m.UpdatedAt,
	}, nil
}
