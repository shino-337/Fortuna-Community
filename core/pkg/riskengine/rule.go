package riskengine

// Types are now defined in types.go to avoid import cycles
// This file contains GetBuiltInRules() function for backward compatibility
// NOTE: YAML rules are now the primary source. Hardcoded rules serve as fallback.

// GetBuiltInRules returns the built-in risk rules (fallback when YAML not available)
// These rules are now also available as YAML files in core/rules/
func GetBuiltInRules() []Rule {
	return []Rule{
		// CIS 5.1.3: Cluster-Admin Binding
		{
			ID:          "cis-5.1.3",
			Name:        "ServiceAccount granted cluster-admin",
			Category:    CategoryRBAC,
			Severity:    SeverityCritical,
			Description: "A ServiceAccount is bound to cluster-admin role, granting full cluster access",
			Enabled:     true,
			Conditions: []Condition{
				{
					Type:     CondTypeResource,
					Field:    "roleRef.name",
					Operator: OpEquals,
					Value:    "cluster-admin",
				},
				{
					Type:     CondTypeResource,
					Field:    "subjects[].kind",
					Operator: OpEquals,
					Value:    "ServiceAccount",
				},
			},
			Aggregation: AggregationAND,
			BaseScore:   10.0,
			Tags:        []string{"cis", "cluster-admin", "critical"},
		},
		// Wildcard Permissions
		{
			ID:          "wildcard-permissions",
			Name:        "Role with wildcard permissions",
			Category:    CategoryRBAC,
			Severity:    SeverityHigh,
			Description: "Role or ClusterRole contains wildcard (*) permissions",
			Enabled:     true,
			Conditions: []Condition{
				{
					Type:       CondTypeExpression,
					Expression: "rules[].resources contains '*' || rules[].verbs contains '*'",
				},
			},
			Aggregation: AggregationOR,
			BaseScore:   8.0,
			Tags:        []string{"wildcard", "overprivileged"},
		},
		// Orphan ServiceAccount
		{
			ID:          "orphan-serviceaccount",
			Name:        "Orphan ServiceAccount (not used by any pod)",
			Category:    CategoryRBAC,
			Severity:    SeverityLow,
			Description: "ServiceAccount exists but is not used by any pod",
			Enabled:     true,
			Conditions: []Condition{
				{
					Type:     CondTypeResource,
					Field:    "linkedPods",
					Operator: OpEquals,
					Value:    "[]",
				},
			},
			Aggregation: AggregationAND,
			BaseScore:   2.0,
			Tags:        []string{"orphan", "unused"},
		},
		// Overprivileged Role
		{
			ID:          "overprivileged-role",
			Name:        "Role with excessive permissions",
			Category:    CategoryRBAC,
			Severity:    SeverityHigh,
			Description: "Role grants permissions to sensitive resources (secrets, configmaps, pods)",
			Enabled:     true,
			Conditions: []Condition{
				{
					Type:     CondTypeResource,
					Field:    "rules[].resources",
					Operator: OpIn,
					Value:    []string{"secrets", "configmaps", "pods"},
				},
				{
					Type:     CondTypeResource,
					Field:    "rules[].verbs",
					Operator: OpIn,
					Value:    []string{"*", "create", "update", "delete"},
				},
			},
			Aggregation: AggregationAND,
			BaseScore:   7.0,
			Tags:        []string{"overprivileged", "sensitive-resources"},
		},
		// Overprivileged Binding
		{
			ID:          "overprivileged-binding",
			Name:        "ServiceAccount bound to overprivileged role",
			Category:    CategoryRBAC,
			Severity:    SeverityHigh,
			Description: "ServiceAccount is bound to a role with excessive permissions",
			Enabled:     true,
			Conditions: []Condition{
				{
					Type:       CondTypeExpression,
					Expression: "roleRef.kind == 'Role' || roleRef.kind == 'ClusterRole'",
				},
			},
			Aggregation: AggregationAND,
			BaseScore:   6.0,
			Tags:        []string{"overprivileged", "binding"},
		},
	}
}
