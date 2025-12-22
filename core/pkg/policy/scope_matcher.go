package policy

import (
	"encoding/json"
	"strings"

	"github.com/ksam/core/pkg/models"
)

// ScopeMatcher handles scope matching logic for policy instances
type ScopeMatcher struct{}

// NewScopeMatcher creates a new scope matcher
func NewScopeMatcher() *ScopeMatcher {
	return &ScopeMatcher{}
}

// Matches checks if a resource matches the instance scope
func (sm *ScopeMatcher) Matches(instance *models.PolicyInstance, resource *Resource) bool {
	// Check clusters (pattern matching)
	if len(instance.Clusters) > 0 {
		if !sm.matchesPatterns(instance.Clusters, resource.ClusterID) {
			return false
		}
	}

	// Check namespaces (pattern matching)
	if len(instance.Namespaces) > 0 {
		if !sm.matchesPatterns(instance.Namespaces, resource.Namespace) {
			return false
		}
	}

	// Check resource types (exact match)
	if len(instance.ResourceTypes) > 0 {
		if !sm.contains(instance.ResourceTypes, resource.Type) {
			return false
		}
	}

	// Check label selectors (if specified)
	if instance.LabelSelectors != "" && instance.LabelSelectors != "{}" {
		if !sm.matchesLabels(instance.LabelSelectors, resource.Labels) {
			return false
		}
	}

	return true
}

// matchesPatterns checks if value matches any pattern in patterns
func (sm *ScopeMatcher) matchesPatterns(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if sm.matchesPattern(pattern, value) {
			return true
		}
	}
	return false
}

// matchesPattern checks if value matches pattern (supports wildcards)
func (sm *ScopeMatcher) matchesPattern(pattern, value string) bool {
	// Exact match
	if pattern == value {
		return true
	}

	// Wildcard matching: "prod-*" matches "prod-cluster-1"
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}

	// Prefix matching: "*prod" matches "my-prod-cluster"
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(value, suffix)
	}

	// Contains matching: "*prod*" matches "my-prod-cluster"
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		substr := strings.TrimPrefix(strings.TrimSuffix(pattern, "*"), "*")
		return strings.Contains(value, substr)
	}

	return false
}

// contains checks if slice contains value
func (sm *ScopeMatcher) contains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}

// matchesLabels checks if resource labels match label selectors
func (sm *ScopeMatcher) matchesLabels(selectorsJSON string, resourceLabels map[string]string) bool {
	if selectorsJSON == "" || selectorsJSON == "{}" {
		return true
	}

	var selectors map[string]string
	if err := json.Unmarshal([]byte(selectorsJSON), &selectors); err != nil {
		// If parsing fails, skip label matching
		return true
	}

	// All selectors must match
	for key, value := range selectors {
		if resourceValue, ok := resourceLabels[key]; !ok || resourceValue != value {
			return false
		}
	}

	return true
}

