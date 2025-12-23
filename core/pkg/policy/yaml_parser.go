package policy

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/fortuna/core/pkg/models"
)

// YAMLParser parses YAML files for Policy Templates and Instances
type YAMLParser struct {
	templatesDir string
	instancesDir string
}

// NewYAMLParser creates a new YAML parser
func NewYAMLParser(templatesDir, instancesDir string) *YAMLParser {
	return &YAMLParser{
		templatesDir: templatesDir,
		instancesDir: instancesDir,
	}
}

// TemplateYAML represents a Policy Template in YAML format
type TemplateYAML struct {
	TemplateID    string                 `yaml:"templateId"`
	Version       string                 `yaml:"version"`
	Name          string                 `yaml:"name"`
	Description   string                 `yaml:"description"`
	Category      string                 `yaml:"category"` // security, compliance, operational, governance
	DefaultSeverity string               `yaml:"defaultSeverity"`
	CELExpression string                 `yaml:"celExpression"`
	DefaultScope  map[string]interface{} `yaml:"defaultScope,omitempty"`
	DefaultAction string                 `yaml:"defaultAction"` // alert, block, audit
	SupportsRemediation bool             `yaml:"supportsRemediation,omitempty"`
	RemediationTemplate map[string]interface{} `yaml:"remediationTemplate,omitempty"`
	Rationale     string                 `yaml:"rationale,omitempty"`
	References    []string               `yaml:"references,omitempty"`
	Examples      []map[string]interface{} `yaml:"examples,omitempty"`
	IsSystem      bool                   `yaml:"isSystem,omitempty"`
}

// InstanceYAML represents a Policy Instance in YAML format
type InstanceYAML struct {
	TemplateID      string                 `yaml:"templateId"`
	TemplateVersion string                 `yaml:"templateVersion"`
	InstanceName    string                 `yaml:"instanceName"`
	Description     string                 `yaml:"description,omitempty"`
	Enabled         bool                   `yaml:"enabled,omitempty"`
	Clusters        []string               `yaml:"clusters,omitempty"`
	Namespaces      []string               `yaml:"namespaces,omitempty"`
	ResourceTypes   []string               `yaml:"resourceTypes,omitempty"`
	LabelSelectors  map[string]string      `yaml:"labelSelectors,omitempty"`
	Action          string                 `yaml:"action,omitempty"` // Override default action
	Severity        string                 `yaml:"severity,omitempty"` // Override default severity
	CustomMessage   string                 `yaml:"customMessage,omitempty"`
	AutoRemediate   bool                   `yaml:"autoRemediate,omitempty"`
	RemediationDryRun bool                 `yaml:"remediationDryRun,omitempty"`
	Exemptions      []map[string]interface{} `yaml:"exemptions,omitempty"`
}

// LoadTemplatesFromYAML loads all Policy Templates from YAML files
func (p *YAMLParser) LoadTemplatesFromYAML() ([]*models.PolicyTemplate, error) {
	if p.templatesDir == "" {
		return nil, fmt.Errorf("templates directory not specified")
	}

	templates := []*models.PolicyTemplate{}

	err := filepath.WalkDir(p.templatesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only process .yaml and .yml files
		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(path), ".yaml") &&
			!strings.HasSuffix(strings.ToLower(path), ".yml") {
			return nil
		}

		// Read and parse YAML file
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[YAMLParser] Failed to read template file %s: %v", path, err)
			return nil // Continue with other files
		}

		var templateYAML TemplateYAML
		if err := yaml.Unmarshal(data, &templateYAML); err != nil {
			log.Printf("[YAMLParser] Failed to parse template file %s: %v", path, err)
			return nil // Continue with other files
		}

		// Validate template
		if err := p.validateTemplate(&templateYAML); err != nil {
			log.Printf("[YAMLParser] Invalid template in %s: %v", path, err)
			return nil // Continue with other files
		}

		// Convert to model
		template := p.convertTemplateToModel(&templateYAML)
		templates = append(templates, template)

		log.Printf("[YAMLParser] Loaded template %s:%s from %s", template.TemplateID, template.Version, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk templates directory: %w", err)
	}

	log.Printf("[YAMLParser] Loaded %d templates from YAML files", len(templates))
	return templates, nil
}

// LoadInstancesFromYAML loads all Policy Instances from YAML files
func (p *YAMLParser) LoadInstancesFromYAML() ([]*models.PolicyInstance, error) {
	if p.instancesDir == "" {
		return nil, fmt.Errorf("instances directory not specified")
	}

	instances := []*models.PolicyInstance{}

	err := filepath.WalkDir(p.instancesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only process .yaml and .yml files
		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(path), ".yaml") &&
			!strings.HasSuffix(strings.ToLower(path), ".yml") {
			return nil
		}

		// Read and parse YAML file
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[YAMLParser] Failed to read instance file %s: %v", path, err)
			return nil // Continue with other files
		}

		var instanceYAML InstanceYAML
		if err := yaml.Unmarshal(data, &instanceYAML); err != nil {
			log.Printf("[YAMLParser] Failed to parse instance file %s: %v", path, err)
			return nil // Continue with other files
		}

		// Validate instance
		if err := p.validateInstance(&instanceYAML); err != nil {
			log.Printf("[YAMLParser] Invalid instance in %s: %v", path, err)
			return nil // Continue with other files
		}

		// Convert to model
		instance := p.convertInstanceToModel(&instanceYAML)
		instances = append(instances, instance)

		log.Printf("[YAMLParser] Loaded instance %s from %s", instance.InstanceName, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk instances directory: %w", err)
	}

	log.Printf("[YAMLParser] Loaded %d instances from YAML files", len(instances))
	return instances, nil
}

// validateTemplate validates a TemplateYAML structure
func (p *YAMLParser) validateTemplate(t *TemplateYAML) error {
	if t.TemplateID == "" {
		return fmt.Errorf("templateId is required")
	}
	if t.Version == "" {
		return fmt.Errorf("version is required")
	}
	if t.Name == "" {
		return fmt.Errorf("name is required")
	}
	if t.Category == "" {
		return fmt.Errorf("category is required")
	}
	if !isValidCategory(t.Category) {
		return fmt.Errorf("invalid category: %s (must be: security, compliance, operational, governance)", t.Category)
	}
	if t.DefaultSeverity == "" {
		return fmt.Errorf("defaultSeverity is required")
	}
	if t.CELExpression == "" {
		return fmt.Errorf("celExpression is required")
	}
	if t.DefaultAction != "" && !isValidAction(t.DefaultAction) {
		return fmt.Errorf("invalid defaultAction: %s (must be: alert, block, audit)", t.DefaultAction)
	}
	return nil
}

// validateInstance validates an InstanceYAML structure
func (p *YAMLParser) validateInstance(i *InstanceYAML) error {
	if i.TemplateID == "" {
		return fmt.Errorf("templateId is required")
	}
	if i.TemplateVersion == "" {
		return fmt.Errorf("templateVersion is required")
	}
	if i.InstanceName == "" {
		return fmt.Errorf("instanceName is required")
	}
	if i.Action != "" && !isValidAction(i.Action) {
		return fmt.Errorf("invalid action: %s (must be: alert, block, audit, remediate)", i.Action)
	}
	if i.Severity != "" && !isValidSeverity(i.Severity) {
		return fmt.Errorf("invalid severity: %s (must be: low, medium, high, critical)", i.Severity)
	}
	return nil
}

// convertTemplateToModel converts TemplateYAML to PolicyTemplate model
func (p *YAMLParser) convertTemplateToModel(t *TemplateYAML) *models.PolicyTemplate {
	// Convert defaultScope to JSON string
	defaultScopeJSON := "{}"
	if t.DefaultScope != nil {
		if data, err := yaml.Marshal(t.DefaultScope); err == nil {
			// Convert YAML to JSON (simple approach: use YAML as JSON-compatible)
			defaultScopeJSON = string(data)
		}
	}

	// Convert remediationTemplate to JSON string
	remediationJSON := "{}"
	if t.RemediationTemplate != nil {
		if data, err := yaml.Marshal(t.RemediationTemplate); err == nil {
			remediationJSON = string(data)
		}
	}

	// Convert examples to JSON string
	examplesJSON := "[]"
	if t.Examples != nil && len(t.Examples) > 0 {
		if data, err := yaml.Marshal(t.Examples); err == nil {
			examplesJSON = string(data)
		}
	}

	template := &models.PolicyTemplate{
		TemplateID:         t.TemplateID,
		Version:            t.Version,
		Name:               t.Name,
		Description:        t.Description,
		Category:           t.Category,
		DefaultSeverity:    t.DefaultSeverity,
		CELExpression:      t.CELExpression,
		DefaultScope:       defaultScopeJSON,
		DefaultAction:      t.DefaultAction,
		SupportsRemediation: t.SupportsRemediation,
		RemediationTemplate: remediationJSON,
		Rationale:          t.Rationale,
		References:        t.References,
		Examples:          examplesJSON,
		IsSystem:          t.IsSystem,
		CreatedBy:         "system",
	}

	if template.DefaultAction == "" {
		template.DefaultAction = "alert"
	}
	if !template.IsSystem {
		template.IsSystem = true // Default to system templates
	}

	return template
}

// convertInstanceToModel converts InstanceYAML to PolicyInstance model
func (p *YAMLParser) convertInstanceToModel(i *InstanceYAML) *models.PolicyInstance {
	// Convert labelSelectors to JSON string
	labelSelectorsJSON := "{}"
	if i.LabelSelectors != nil && len(i.LabelSelectors) > 0 {
		if data, err := yaml.Marshal(i.LabelSelectors); err == nil {
			labelSelectorsJSON = string(data)
		}
	}

	// Convert exemptions to JSON string
	exemptionsJSON := "[]"
	if i.Exemptions != nil && len(i.Exemptions) > 0 {
		if data, err := yaml.Marshal(i.Exemptions); err == nil {
			exemptionsJSON = string(data)
		}
	}

	instance := &models.PolicyInstance{
		TemplateID:        i.TemplateID,
		TemplateVersion:   i.TemplateVersion,
		InstanceName:      i.InstanceName,
		Description:       i.Description,
		Enabled:           i.Enabled,
		Clusters:          i.Clusters,
		Namespaces:        i.Namespaces,
		ResourceTypes:     i.ResourceTypes,
		LabelSelectors:    labelSelectorsJSON,
		Action:            i.Action,
		Severity:          i.Severity,
		CustomMessage:     i.CustomMessage,
		AutoRemediate:     i.AutoRemediate,
		RemediationDryRun: i.RemediationDryRun,
		Exemptions:        exemptionsJSON,
		CreatedBy:         "system",
	}

	if !instance.Enabled {
		instance.Enabled = true // Default to enabled
	}

	return instance
}

// Helper validation functions
func isValidCategory(category string) bool {
	valid := []string{"security", "compliance", "operational", "governance"}
	for _, v := range valid {
		if category == v {
			return true
		}
	}
	return false
}

func isValidAction(action string) bool {
	valid := []string{"alert", "block", "audit", "remediate"}
	for _, v := range valid {
		if action == v {
			return true
		}
	}
	return false
}

func isValidSeverity(severity string) bool {
	valid := []string{"low", "medium", "high", "critical"}
	for _, v := range valid {
		if severity == v {
			return true
		}
	}
	return false
}

