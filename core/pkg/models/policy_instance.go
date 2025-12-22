package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// PolicyInstance represents a user-configurable policy instance
type PolicyInstance struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Template reference (immutable after creation)
	TemplateID      string `gorm:"not null" json:"templateId"`
	TemplateVersion string `gorm:"not null" json:"templateVersion"`

	// Instance identity
	InstanceName string `gorm:"uniqueIndex;not null" json:"instanceName"`
	Description  string `gorm:"type:text" json:"description"`

	// Status
	Enabled bool `gorm:"default:true" json:"enabled"`

	// Scope configuration (USER CONFIGURABLE)
	Clusters       StringArray `gorm:"type:text[]" json:"clusters"`      // ["prod-*", "staging-*"]
	Namespaces     StringArray `gorm:"type:text[]" json:"namespaces"`    // ["default", "production"]
	ResourceTypes  StringArray `gorm:"type:text[]" json:"resourceTypes"` // ["Pod", "Deployment"]
	LabelSelectors string      `gorm:"type:jsonb" json:"labelSelectors"` // JSON string: {"env": "production"}

	// Action override (USER CONFIGURABLE)
	Action   string `gorm:"check:action IN ('alert', 'block', 'audit', 'remediate')" json:"action"`  // Override default action
	Severity string `gorm:"check:severity IN ('low', 'medium', 'high', 'critical')" json:"severity"` // Override default severity

	// Message override (USER CONFIGURABLE)
	CustomMessage string `gorm:"type:text" json:"customMessage"`

	// Remediation settings (USER CONFIGURABLE)
	AutoRemediate     bool `gorm:"default:false" json:"autoRemediate"`
	RemediationDryRun bool `gorm:"default:true" json:"remediationDryRun"`

	// Exemptions
	Exemptions string `gorm:"type:jsonb" json:"exemptions"` // JSON string: Array of exemption rules

	// Metadata
	CreatedBy string `json:"createdBy"`
	UpdatedBy string `json:"updatedBy"`

	// Relations
	Template PolicyTemplate `gorm:"foreignKey:TemplateID,TemplateVersion;references:TemplateID,Version" json:"template,omitempty"`
}

// TableName specifies the table name for PolicyInstance
func (PolicyInstance) TableName() string {
	return "policy_instances"
}

// StringArray is a custom type for PostgreSQL TEXT[] arrays
type StringArray []string

// Value implements the driver.Valuer interface
func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	// Format as PostgreSQL array: {"value1","value2"}
	values := make([]string, len(a))
	for i, v := range a {
		values[i] = `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
	}
	return "{" + strings.Join(values, ",") + "}", nil
}

// Scan implements the sql.Scanner interface
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = []string{}
		return nil
	}

	var str string
	switch v := value.(type) {
	case []byte:
		str = string(v)
	case string:
		str = v
	default:
		return errors.New("cannot scan non-string value into StringArray")
	}

	// Parse PostgreSQL array format: {value1,value2} or {"value1","value2"}
	str = strings.TrimSpace(str)
	if str == "{}" || str == "" {
		*a = []string{}
		return nil
	}

	// Remove braces
	if strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}") {
		str = str[1 : len(str)-1]
	}

	// Split by comma, handling quoted strings
	var result []string
	if str != "" {
		// Simple split for now - can be enhanced for quoted strings with commas
		parts := strings.Split(str, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			// Remove quotes if present
			if strings.HasPrefix(part, `"`) && strings.HasSuffix(part, `"`) {
				part = part[1 : len(part)-1]
				part = strings.ReplaceAll(part, `\"`, `"`)
				part = strings.ReplaceAll(part, `\\`, `\`)
			}
			if part != "" {
				result = append(result, part)
			}
		}
	}

	*a = result
	return nil
}

// MarshalJSON implements json.Marshaler
func (a StringArray) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(a))
}

// UnmarshalJSON implements json.Unmarshaler
func (a *StringArray) UnmarshalJSON(data []byte) error {
	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	*a = StringArray(arr)
	return nil
}
