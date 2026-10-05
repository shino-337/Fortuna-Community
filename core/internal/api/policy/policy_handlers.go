package policy

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/api/listlimit"
	"github.com/fortuna/core/pkg/models"
)

// PolicyHandler handles policy-related API requests
type PolicyHandler struct {
	db *gorm.DB
}

// NewPolicyHandler creates a new policy handler
func NewPolicyHandler(db *gorm.DB) *PolicyHandler {
	return &PolicyHandler{
		db: db,
	}
}

// Policy templates and instances are configuration; the defaults sit at the
// hard maximum so the Rules page keeps showing every row.
const (
	policyListDefaultLimit = 1000
	policyListMaxLimit     = 1000
)

// ==================== Policy Templates ====================

// ListTemplates lists all policy templates
func (h *PolicyHandler) ListTemplates(c *gin.Context) {
	var templates []models.PolicyTemplate

	query := h.db.Where("deleted_at IS NULL")

	// Filter by category
	if category := c.Query("category"); category != "" {
		query = query.Where("category = ?", category)
	}

	// Filter by is_system
	if isSystem := c.Query("isSystem"); isSystem != "" {
		isSystemBool, _ := strconv.ParseBool(isSystem)
		query = query.Where("is_system = ?", isSystemBool)
	}

	limit := listlimit.Parse(c, policyListDefaultLimit, policyListMaxLimit)
	if err := query.Order("template_id, version").Limit(limit + 1).Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}
	templates, truncated := listlimit.Trim(templates, limit)

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
		"truncated": truncated,
	})
}

// GetTemplate gets a specific policy template
func (h *PolicyHandler) GetTemplate(c *gin.Context) {
	templateID := c.Param("templateId")
	version := c.Query("version")

	var template models.PolicyTemplate
	query := h.db.Where("template_id = ? AND deleted_at IS NULL", templateID)

	if version != "" {
		query = query.Where("version = ?", version)
	} else {
		// Get latest version if not specified
		query = query.Order("version DESC").Limit(1)
	}

	if err := query.First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// CreateTemplate creates a new policy template
func (h *PolicyHandler) CreateTemplate(c *gin.Context) {
	var template models.PolicyTemplate

	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if template.TemplateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "templateId is required"})
		return
	}
	if template.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}
	if template.CELExpression == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "celExpression is required"})
		return
	}
	if !oneOf(template.Category, templateCategories) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category must be one of security, compliance, operational, governance"})
		return
	}
	if template.DefaultAction != "" && !oneOf(template.DefaultAction, templateActions) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "defaultAction must be one of alert, block, audit"})
		return
	}
	if !oneOf(template.DefaultSeverity, policySeverities) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "defaultSeverity must be one of low, medium, high, critical"})
		return
	}

	// Check if template already exists
	var existing models.PolicyTemplate
	// Unscoped: a deleted template keeps its (templateId, version) because
	// instances and violations refer to it; publish a new version instead.
	if err := h.db.Unscoped().Where("template_id = ? AND version = ?", template.TemplateID, template.Version).
		First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Template already exists"})
		return
	}

	// Set defaults
	if template.DefaultAction == "" {
		template.DefaultAction = "alert"
	}
	if template.CreatedBy == "" {
		template.CreatedBy = "system"
	}
	// Templates created through the API are user templates and can be deleted;
	// only migrations seed system templates.
	template.IsSystem = false

	if err := h.db.Create(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
		return
	}

	c.JSON(http.StatusCreated, template)
}

// UpdateTemplate updates an existing policy template
func (h *PolicyHandler) UpdateTemplate(c *gin.Context) {
	templateID := c.Param("templateId")
	version := c.Param("version")

	var template models.PolicyTemplate
	if err := h.db.Where("template_id = ? AND version = ? AND deleted_at IS NULL", templateID, version).
		First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template"})
		return
	}

	// Only documentation fields can change; fields left out of the body keep
	// their stored value.
	var updates struct {
		Description *string   `json:"description"`
		Rationale   *string   `json:"rationale"`
		References  *[]string `json:"references"`
		Examples    *string   `json:"examples"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if updates.Description != nil {
		template.Description = *updates.Description
	}
	if updates.Rationale != nil {
		template.Rationale = *updates.Rationale
	}
	if updates.References != nil {
		template.References = *updates.References
	}
	if updates.Examples != nil {
		template.Examples = *updates.Examples
	}

	if err := h.db.Save(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update template"})
		return
	}

	c.JSON(http.StatusOK, template)
}

// DeleteTemplate deletes a policy template (soft delete)
func (h *PolicyHandler) DeleteTemplate(c *gin.Context) {
	templateID := c.Param("templateId")
	version := c.Param("version")

	var template models.PolicyTemplate
	if err := h.db.Where("template_id = ? AND version = ? AND deleted_at IS NULL", templateID, version).
		First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get template"})
		return
	}

	// Prevent deletion of system templates
	if template.IsSystem {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete system templates"})
		return
	}

	if err := h.db.Delete(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted successfully"})
}

// ==================== Policy Instances ====================

// ListInstances lists all policy instances
func (h *PolicyHandler) ListInstances(c *gin.Context) {
	var instances []models.PolicyInstance

	query := h.db.Where("deleted_at IS NULL")

	// Filter by template
	if templateID := c.Query("templateId"); templateID != "" {
		query = query.Where("template_id = ?", templateID)
	}

	// Filter by enabled
	if enabled := c.Query("enabled"); enabled != "" {
		enabledBool, _ := strconv.ParseBool(enabled)
		query = query.Where("enabled = ?", enabledBool)
	}

	limit := listlimit.Parse(c, policyListDefaultLimit, policyListMaxLimit)
	if err := query.Order("instance_name").Limit(limit + 1).Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list instances"})
		return
	}
	instances, truncated := listlimit.Trim(instances, limit)

	c.JSON(http.StatusOK, gin.H{
		"instances": instances,
		"count":     len(instances),
		"truncated": truncated,
	})
}

// GetInstance gets a specific policy instance
func (h *PolicyHandler) GetInstance(c *gin.Context) {
	instanceName := c.Param("instanceName")

	var instance models.PolicyInstance
	if err := h.db.Where("instance_name = ? AND deleted_at IS NULL", instanceName).
		First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Instance not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get instance"})
		return
	}

	c.JSON(http.StatusOK, instance)
}

// CreateInstance creates a new policy instance
func (h *PolicyHandler) CreateInstance(c *gin.Context) {
	var instance models.PolicyInstance
	// The flags default to true when omitted, so their presence in the body matters.
	var flags struct {
		Enabled           *bool `json:"enabled"`
		RemediationDryRun *bool `json:"remediationDryRun"`
	}
	if err := c.ShouldBindBodyWith(&instance, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.ShouldBindBodyWith(&flags, binding.JSON); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	instance.Enabled = flags.Enabled == nil || *flags.Enabled
	instance.RemediationDryRun = flags.RemediationDryRun == nil || *flags.RemediationDryRun

	// Validate required fields
	if instance.TemplateID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "templateId is required"})
		return
	}
	if instance.TemplateVersion == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "templateVersion is required"})
		return
	}
	if instance.InstanceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instanceName is required"})
		return
	}

	// Verify template exists
	var template models.PolicyTemplate
	if err := h.db.Where("template_id = ? AND version = ?", instance.TemplateID, instance.TemplateVersion).
		First(&template).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Template not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify template"})
		return
	}

	// Empty action/severity inherit from the template.
	if instance.Action == "" {
		instance.Action = template.DefaultAction
	}
	if instance.Severity == "" {
		instance.Severity = template.DefaultSeverity
	}
	if msg := validateInstanceOverrides(instance.Action, instance.Severity); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check if instance already exists
	var existing models.PolicyInstance
	if err := h.db.Where("instance_name = ?", instance.InstanceName).
		First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Instance already exists"})
		return
	}

	// Set defaults
	if instance.CreatedBy == "" {
		instance.CreatedBy = "system"
	}

	// GORM inserts DEFAULT (true) for a false value in a column with a default
	// and reads the stored value back, so the requested flags are written after.
	enabled, dryRun := instance.Enabled, instance.RemediationDryRun
	err := h.db.Transaction(func(tx *gorm.DB) error {
		// The unique index on instance_name also covers soft-deleted rows, so a
		// deleted instance's name is freed before it is reused.
		if err := tx.Unscoped().Where("instance_name = ? AND deleted_at IS NOT NULL", instance.InstanceName).
			Delete(&models.PolicyInstance{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&instance).Error; err != nil {
			return err
		}
		if !enabled || !dryRun {
			return tx.Model(&instance).Updates(map[string]any{
				"enabled":             enabled,
				"remediation_dry_run": dryRun,
			}).Error
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create instance"})
		return
	}

	c.JSON(http.StatusCreated, instance)
}

// UpdateInstance updates an existing policy instance
func (h *PolicyHandler) UpdateInstance(c *gin.Context) {
	instanceName := c.Param("instanceName")

	var instance models.PolicyInstance
	if err := h.db.Where("instance_name = ? AND deleted_at IS NULL", instanceName).
		First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Instance not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get instance"})
		return
	}

	// Partial update: fields left out of the body keep their stored value. The
	// template reference and instance name cannot change.
	var updates struct {
		Description       *string             `json:"description"`
		Enabled           *bool               `json:"enabled"`
		Clusters          *models.StringArray `json:"clusters"`
		Namespaces        *models.StringArray `json:"namespaces"`
		ResourceTypes     *models.StringArray `json:"resourceTypes"`
		LabelSelectors    *string             `json:"labelSelectors"`
		Action            *string             `json:"action"`
		Severity          *string             `json:"severity"`
		CustomMessage     *string             `json:"customMessage"`
		AutoRemediate     *bool               `json:"autoRemediate"`
		RemediationDryRun *bool               `json:"remediationDryRun"`
		Exemptions        *string             `json:"exemptions"`
		UpdatedBy         *string             `json:"updatedBy"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	setIf(&instance.Description, updates.Description)
	setIf(&instance.Enabled, updates.Enabled)
	setIf(&instance.Clusters, updates.Clusters)
	setIf(&instance.Namespaces, updates.Namespaces)
	setIf(&instance.ResourceTypes, updates.ResourceTypes)
	setIf(&instance.LabelSelectors, updates.LabelSelectors)
	setIf(&instance.Action, updates.Action)
	setIf(&instance.Severity, updates.Severity)
	setIf(&instance.CustomMessage, updates.CustomMessage)
	setIf(&instance.AutoRemediate, updates.AutoRemediate)
	setIf(&instance.RemediationDryRun, updates.RemediationDryRun)
	setIf(&instance.Exemptions, updates.Exemptions)
	setIf(&instance.UpdatedBy, updates.UpdatedBy)
	if msg := validateInstanceOverrides(instance.Action, instance.Severity); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	if err := h.db.Save(&instance).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update instance"})
		return
	}

	c.JSON(http.StatusOK, instance)
}

// DeleteInstance deletes a policy instance (soft delete)
func (h *PolicyHandler) DeleteInstance(c *gin.Context) {
	instanceName := c.Param("instanceName")

	var instance models.PolicyInstance
	if err := h.db.Where("instance_name = ? AND deleted_at IS NULL", instanceName).
		First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Instance not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get instance"})
		return
	}

	if err := h.db.Delete(&instance).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete instance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Instance deleted successfully"})
}

var (
	templateCategories = []string{"security", "compliance", "operational", "governance"}
	templateActions    = []string{"alert", "block", "audit"}
	instanceActions    = []string{"alert", "block", "audit", "remediate"}
	policySeverities   = []string{"low", "medium", "high", "critical"}
)

func oneOf(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

// validateInstanceOverrides mirrors the database check constraints so a bad
// value is a 400 instead of a failed write.
func validateInstanceOverrides(action, severity string) string {
	if !oneOf(action, instanceActions) {
		return "action must be one of alert, block, audit, remediate"
	}
	if !oneOf(severity, policySeverities) {
		return "severity must be one of low, medium, high, critical"
	}
	return ""
}

func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}
