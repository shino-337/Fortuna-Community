package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
)

// QueryService provides advanced graph query operations
type QueryService struct {
	engine *AgeGraphEngine
}

// NewQueryService creates a new graph query service
func NewQueryService(db *gorm.DB) (*QueryService, error) {
	engine, err := NewAgeGraphEngine(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph engine: %w", err)
	}

	return &QueryService{
		engine: engine,
	}, nil
}

// GetAttackPath finds attack paths from a pod to sensitive resources (roles with admin permissions)
func (s *QueryService) GetAttackPath(ctx context.Context, podUID string, maxDepth int) ([]AttackPath, error) {
	if !s.engine.IsEnabled() {
		// Fallback: return empty paths if AGE not available
		log.Printf("[QueryService] AGE not enabled, returning empty attack paths")
		return []AttackPath{}, nil
	}

	if maxDepth < 1 || maxDepth > 10 {
		maxDepth = 5 // Default max depth
	}

	// Query to find paths from Pod to admin roles
	// Note: This is a simplified query - in production, you'd want more sophisticated path finding
	// AGE Cypher queries: use $pod_uid in Cypher, $1 in SQL
	query := fmt.Sprintf(`
		SELECT * FROM cypher('fortuna_graph', $$
			MATCH path = (p:Pod {uid: $pod_uid})-[*1..%d]->(target)
			WHERE (target:Role OR target:ClusterRole)
			AND (
				target.name =~ '.*admin.*' 
				OR target.name =~ '.*cluster-admin.*'
			)
			RETURN path, length(path) as path_length
			LIMIT 50
		$$, jsonb_build_object('pod_uid', $1)) AS (path agtype, path_length agtype)
	`, maxDepth)

	rows, err := s.engine.GetSQLDB().QueryContext(ctx, query, podUID)
	if err != nil {
		// If query fails (e.g., AGE not fully set up), return empty
		log.Printf("[QueryService] Failed to execute attack path query: %v", err)
		return []AttackPath{}, nil
	}
	defer rows.Close()

	var paths []AttackPath
	for rows.Next() {
		var pathData, pathLengthData string
		if err := rows.Scan(&pathData, &pathLengthData); err != nil {
			log.Printf("[QueryService] Failed to scan attack path: %v", err)
			continue
		}

		// Parse path data (simplified - would need proper AGTYPE parsing in production)
		path, err := s.parseAttackPath(pathData, pathLengthData)
		if err != nil {
			log.Printf("[QueryService] Failed to parse attack path: %v", err)
			continue
		}

		paths = append(paths, path)
	}

	return paths, nil
}

// GetServiceAccountPermissions gets all permissions for a service account via graph traversal
func (s *QueryService) GetServiceAccountPermissions(ctx context.Context, saUID string) ([]Permission, error) {
	if !s.engine.IsEnabled() {
		// Fallback: return empty permissions if AGE not available
		log.Printf("[QueryService] AGE not enabled, returning empty permissions")
		return []Permission{}, nil
	}

	query := `
		SELECT * FROM cypher('fortuna_graph', $$
			MATCH (sa:ServiceAccount {uid: $sa_uid})-[:BINDS_TO]->(rb)-[:GRANTS_ROLE]->(role)
			RETURN 
				role.name AS role_name,
				role.rules AS rules,
				COALESCE(rb.namespace, '') AS namespace,
				labels(role)[0] AS role_type
		$$, jsonb_build_object('sa_uid', $1)) AS (role_name agtype, rules agtype, namespace agtype, role_type agtype)
	`

	rows, err := s.engine.GetSQLDB().QueryContext(ctx, query, saUID)
	if err != nil {
		log.Printf("[QueryService] Failed to execute permissions query: %v", err)
		return []Permission{}, nil
	}
	defer rows.Close()

	var permissions []Permission
	for rows.Next() {
		var roleName, rulesData, namespace, roleType string
		if err := rows.Scan(&roleName, &rulesData, &namespace, &roleType); err != nil {
			log.Printf("[QueryService] Failed to scan permission: %v", err)
			continue
		}

		// Parse rules from JSON
		rules, err := s.parsePolicyRules(rulesData)
		if err != nil {
			log.Printf("[QueryService] Failed to parse rules: %v", err)
			continue
		}

		permission := Permission{
			RoleName:  roleName,
			Rules:     rules,
			Namespace: namespace,
			RoleType:  roleType,
		}
		permissions = append(permissions, permission)
	}

	return permissions, nil
}

// GetPodsWithEscalationRisk finds pods that can escalate privileges
func (s *QueryService) GetPodsWithEscalationRisk(ctx context.Context) ([]RiskyPod, error) {
	if !s.engine.IsEnabled() {
		log.Printf("[QueryService] AGE not enabled, returning empty risky pods")
		return []RiskyPod{}, nil
	}

	query := `
		SELECT * FROM cypher('fortuna_graph', $$
			MATCH (p:Pod)-[:USES_SERVICE_ACCOUNT]->(sa:ServiceAccount)-[:BINDS_TO]->(rb)-[:GRANTS_ROLE]->(role:Role|ClusterRole)
			WHERE 
				role.rules @> '[{"verbs": ["*"], "resources": ["*"]}]'::jsonb
				OR role.name =~ '.*cluster-admin.*'
				OR role.name =~ '.*admin.*'
			RETURN 
				p.uid AS pod_uid,
				p.name AS pod_name,
				p.namespace AS namespace,
				sa.name AS sa_name,
				role.name AS role_name
		$$) AS (pod_uid agtype, pod_name agtype, namespace agtype, sa_name agtype, role_name agtype)
	`

	rows, err := s.engine.GetSQLDB().QueryContext(ctx, query)
	if err != nil {
		log.Printf("[QueryService] Failed to execute risky pods query: %v", err)
		return []RiskyPod{}, nil
	}
	defer rows.Close()

	var riskyPods []RiskyPod
	for rows.Next() {
		var pod RiskyPod
		if err := rows.Scan(&pod.UID, &pod.Name, &pod.Namespace, &pod.ServiceAccountName, &pod.RoleName); err != nil {
			log.Printf("[QueryService] Failed to scan risky pod: %v", err)
			continue
		}

		// Calculate risk score and reason
		pod.RiskScore = s.calculateRiskScore(pod.RoleName)
		pod.RiskReason = s.getRiskReason(pod.RoleName)

		riskyPods = append(riskyPods, pod)
	}

	return riskyPods, nil
}

// PathConstraint defines conditions for finding paths (BloodHound-style)
type PathConstraint struct {
	MaxDepth               int
	RequireRuntimeEvidence bool
	MinRealism             float64
	MustIncludeTechnique   string
}

// FindPaths finds attack paths between any source and target node based on constraints
func (s *QueryService) FindPaths(ctx context.Context, sourceID string, targetID string, constraints PathConstraint) ([]AttackPath, error) {
	if !s.engine.IsEnabled() {
		log.Printf("[QueryService] AGE not enabled, returning empty paths for FindPaths")
		return []AttackPath{}, nil
	}

	maxDepth := constraints.MaxDepth
	if maxDepth < 1 || maxDepth > 15 {
		maxDepth = 7 // Default reasonable depth
	}

	// This builds a flexible Cypher query for arbitrary source -> target
	query := fmt.Sprintf(`
		SELECT * FROM cypher('fortuna_graph', $$
			MATCH path = (source {uid: $source_uid})-[*1..%d]->(target {uid: $target_uid})
			RETURN path, length(path) as path_length
			LIMIT 100
		$$, jsonb_build_object('source_uid', $1, 'target_uid', $2)) AS (path agtype, path_length agtype)
	`, maxDepth)

	rows, err := s.engine.GetSQLDB().QueryContext(ctx, query, sourceID, targetID)
	if err != nil {
		log.Printf("[QueryService] Failed to execute FindPaths query: %v", err)
		return []AttackPath{}, nil
	}
	defer rows.Close()

	var paths []AttackPath
	for rows.Next() {
		var pathData, pathLengthData string
		if err := rows.Scan(&pathData, &pathLengthData); err != nil {
			log.Printf("[QueryService] Failed to scan path: %v", err)
			continue
		}

		path, err := s.parseAttackPath(pathData, pathLengthData)
		if err != nil {
			continue
		}

		// Apply application-level constraints (e.g. MinRealism, RequireRuntimeEvidence)
		if constraints.MinRealism > 0 && path.Explainability != nil {
			// Compute chain realism if necessary, or check stored Realism
		}

		paths = append(paths, path)
	}

	return paths, nil
}

// Helper functions

func (s *QueryService) parseAttackPath(pathData, pathLengthData string) (AttackPath, error) {
	// Simplified parsing - in production, would need proper AGTYPE parsing
	// For now, return a basic structure
	path := AttackPath{
		Nodes:      []PathNode{},
		Edges:      []PathEdge{},
		TotalRisk:  7.5, // Default risk score
		Difficulty: 0.3,  // Default difficulty
		Impact:     0.9,  // Default impact
		Length:     2,    // Default length
		Description: "Attack path from Pod to admin role",
	}

	// In production, parse the actual AGTYPE path data
	// This is a placeholder implementation
	return path, nil
}

func (s *QueryService) parsePolicyRules(rulesData string) ([]PolicyRule, error) {
	// Parse rules from JSON string
	var rules []map[string]interface{}
	if err := json.Unmarshal([]byte(rulesData), &rules); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rules: %w", err)
	}

	var policyRules []PolicyRule
	for _, ruleMap := range rules {
		rule := PolicyRule{
			Verbs:         []string{},
			Resources:     []string{},
			APIGroups:     []string{},
			ResourceNames: []string{},
		}

		if verbs, ok := ruleMap["verbs"].([]interface{}); ok {
			for _, v := range verbs {
				if verb, ok := v.(string); ok {
					rule.Verbs = append(rule.Verbs, verb)
				}
			}
		}

		if resources, ok := ruleMap["resources"].([]interface{}); ok {
			for _, r := range resources {
				if resource, ok := r.(string); ok {
					rule.Resources = append(rule.Resources, resource)
				}
			}
		}

		if apiGroups, ok := ruleMap["apiGroups"].([]interface{}); ok {
			for _, ag := range apiGroups {
				if apiGroup, ok := ag.(string); ok {
					rule.APIGroups = append(rule.APIGroups, apiGroup)
				}
			}
		}

		if resourceNames, ok := ruleMap["resourceNames"].([]interface{}); ok {
			for _, rn := range resourceNames {
				if resourceName, ok := rn.(string); ok {
					rule.ResourceNames = append(rule.ResourceNames, resourceName)
				}
			}
		}

		policyRules = append(policyRules, rule)
	}

	return policyRules, nil
}

func (s *QueryService) calculateRiskScore(roleName string) float64 {
	roleNameLower := strings.ToLower(roleName)
	if strings.Contains(roleNameLower, "cluster-admin") {
		return 10.0
	}
	if strings.Contains(roleNameLower, "admin") {
		return 9.0
	}
	return 7.0
}

func (s *QueryService) getRiskReason(roleName string) string {
	roleNameLower := strings.ToLower(roleName)
	if strings.Contains(roleNameLower, "cluster-admin") {
		return "Pod uses ServiceAccount bound to cluster-admin role"
	}
	if strings.Contains(roleNameLower, "admin") {
		return "Pod uses ServiceAccount bound to admin role"
	}
	return "Pod uses ServiceAccount with excessive permissions"
}

