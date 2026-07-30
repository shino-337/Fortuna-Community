package riskengine

import (
	"fmt"
	"regexp"
	"strings"
)

// RuleIDPattern: lowercase, digits, hyphens; 1-128 chars (no spaces).
var ruleIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,127}$`)

// ValidateRule returns a list of validation errors (empty if valid). Used by API validate endpoint and create/update.
func ValidateRule(rule *Rule) []string {
	var errs []string
	if rule == nil {
		return []string{"rule is required"}
	}
	if rule.ID == "" {
		errs = append(errs, "id: rule ID is required")
	} else {
		if len(rule.ID) > 128 {
			errs = append(errs, "id: must be at most 128 characters")
		}
		if !ruleIDPattern.MatchString(rule.ID) {
			errs = append(errs, "id: must be lowercase letters, digits, and hyphens only (e.g. my-custom-rule)")
		}
	}
	if strings.TrimSpace(rule.Name) == "" {
		errs = append(errs, "name: rule name is required")
	}
	validSeverities := map[Severity]struct{}{
		SeverityCritical: {}, SeverityHigh: {}, SeverityMedium: {}, SeverityLow: {}, SeverityInfo: {},
	}
	if _, ok := validSeverities[rule.Severity]; !ok {
		errs = append(errs, "severity: must be one of critical, high, medium, low, info")
	}
	validCategories := map[RuleCategory]struct{}{
		CategoryRBAC: {}, CategoryPodSecurity: {}, CategoryNetworkPolicy: {},
		CategorySecrets: {}, CategoryRuntime: {}, CategoryCompliance: {},
	}
	if _, ok := validCategories[rule.Category]; !ok {
		errs = append(errs, "category: must be one of rbac, pod-security, network-policy, secrets, runtime-behavior, compliance")
	}
	if rule.BaseScore < 0 || rule.BaseScore > 10 {
		errs = append(errs, "base_score: must be between 0 and 10")
	}
	validAgg := map[AggregationType]struct{}{
		AggregationAND: {}, AggregationOR: {}, AggregationTHRESHOLD: {},
	}
	if rule.Aggregation != "" {
		if _, ok := validAgg[rule.Aggregation]; !ok {
			errs = append(errs, "aggregation: must be AND, OR, or THRESHOLD")
		}
	}
	if len(rule.Conditions) == 0 {
		errs = append(errs, "conditions: at least one condition is required")
	} else {
		for i, c := range rule.Conditions {
			prefix := fmt.Sprintf("conditions[%d]", i)
			switch c.Type {
			case CondTypeExpression:
				if strings.TrimSpace(c.Expression) == "" {
					errs = append(errs, prefix+": expression is required when type is expression")
				}
			case CondTypeResource:
				if strings.TrimSpace(c.Field) == "" {
					errs = append(errs, prefix+": field is required when type is resource")
				}
				if c.Operator == "" {
					errs = append(errs, prefix+": operator is required when type is resource")
				}
			default:
				if c.Type == "" {
					errs = append(errs, prefix+": type must be expression or resource")
				} else {
					errs = append(errs, prefix+": type must be expression or resource (got "+string(c.Type)+")")
				}
			}
		}
	}
	return errs
}
