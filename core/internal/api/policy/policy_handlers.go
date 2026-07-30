package policy

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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

	if err := query.Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"count":     len(templates),
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

	// Check if template already exists
	var existing models.PolicyTemplate
	if err := h.db.Where("template_id = ? AND version = ?", template.TemplateID, template.Version).
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
	// Respect JSON `isSystem` (defaults to false when omitted). DB column default:true
	// still applies only when the zero value is inserted as DEFAULT; do not force true here.

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

	// Prevent updates to immutable fields
	var updates models.PolicyTemplate
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Only allow updates to certain fields
	template.Description = updates.Description
	template.Rationale = updates.Rationale
	template.References = updates.References
	template.Examples = updates.Examples

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

	if err := query.Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list instances"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"instances": instances,
		"count":     len(instances),
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

	if err := c.ShouldBindJSON(&instance); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

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

	if err := h.db.Create(&instance).Error; err != nil {
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

	var updates models.PolicyInstance
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update allowed fields
	instance.Description = updates.Description
	instance.Enabled = updates.Enabled
	instance.Clusters = updates.Clusters
	instance.Namespaces = updates.Namespaces
	instance.ResourceTypes = updates.ResourceTypes
	instance.LabelSelectors = updates.LabelSelectors
	instance.Action = updates.Action
	instance.Severity = updates.Severity
	instance.CustomMessage = updates.CustomMessage
	instance.AutoRemediate = updates.AutoRemediate
	instance.RemediationDryRun = updates.RemediationDryRun
	instance.Exemptions = updates.Exemptions
	instance.UpdatedBy = updates.UpdatedBy

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

