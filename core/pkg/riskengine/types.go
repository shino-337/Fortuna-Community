package riskengine

// Rule represents a risk evaluation rule
type Rule struct {
	ID          string         `yaml:"id" json:"id"`
	Name        string         `yaml:"name" json:"name"`
	Category    RuleCategory  `yaml:"category" json:"category"`
	Severity    Severity       `yaml:"severity" json:"severity"`
	Description string         `yaml:"description" json:"description"`
	Enabled     bool           `yaml:"enabled" json:"enabled"`
	
	// Rule evaluation
	Conditions  []Condition    `yaml:"conditions" json:"conditions"`
	Aggregation AggregationType `yaml:"aggregation" json:"aggregation"` // AND, OR, THRESHOLD
	
	// Scoring
	BaseScore   float64       `yaml:"base_score" json:"base_score"`    // 0-10 (CVSS-like)
	
	// Metadata
	Tags        []string      `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// RuleCategory represents the category of a rule
type RuleCategory string

const (
	CategoryRBAC          RuleCategory = "rbac"
	CategoryNetworkPolicy RuleCategory = "network-policy"
	CategoryPodSecurity   RuleCategory = "pod-security"
	CategorySecrets       RuleCategory = "secrets"
	CategoryRuntime       RuleCategory = "runtime-behavior"
	CategoryCompliance    RuleCategory = "compliance"
)

// Severity represents the severity level
type Severity string

const (
	SeverityCritical Severity = "critical" // CVSS 9.0-10.0
	SeverityHigh     Severity = "high"     // CVSS 7.0-8.9
	SeverityMedium   Severity = "medium"   // CVSS 4.0-6.9
	SeverityLow      Severity = "low"      // CVSS 0.1-3.9
	SeverityInfo     Severity = "info"     // Informational
)

// Condition represents a rule condition
type Condition struct {
	Type      ConditionType      `yaml:"type" json:"type"`
	Field     string             `yaml:"field,omitempty" json:"field,omitempty"`
	Operator  ComparisonOperator `yaml:"operator,omitempty" json:"operator,omitempty"`
	Value     interface{}        `yaml:"value,omitempty" json:"value,omitempty"`
	Expression string            `yaml:"expression,omitempty" json:"expression,omitempty"` // Simple expression
}

// ConditionType represents the type of condition
type ConditionType string

const (
	CondTypeResource   ConditionType = "resource"      // K8s resource field
	CondTypeExpression ConditionType = "expression"    // Simple expression
)

// ComparisonOperator represents comparison operators
type ComparisonOperator string

const (
	OpEquals      ComparisonOperator = "eq"
	OpNotEquals   ComparisonOperator = "ne"
	OpContains    ComparisonOperator = "contains"
	OpMatches     ComparisonOperator = "matches" // Regex
	OpExists      ComparisonOperator = "exists"
	OpIn          ComparisonOperator = "in"
)

// AggregationType represents how conditions are aggregated
type AggregationType string

const (
	AggregationAND       AggregationType = "AND"
	AggregationOR        AggregationType = "OR"
	AggregationTHRESHOLD AggregationType = "THRESHOLD"
)

