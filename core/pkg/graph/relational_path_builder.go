package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// RelationalPathBuilder computes attack paths using relational (SQL) queries
// without requiring Apache AGE. It resolves:
//
//	Pod → ServiceAccount → RoleBinding/ClusterRoleBinding → Role/ClusterRole
//
// and scores each path based on the target role's permissions and the pod's
// security posture (hostNetwork, hostPID, capabilities, etc.).
type RelationalPathBuilder struct {
	db *gorm.DB
}

// NewRelationalPathBuilder creates a new relational path builder.
func NewRelationalPathBuilder(db *gorm.DB) *RelationalPathBuilder {
	return &RelationalPathBuilder{db: db}
}

// roleRef is the shape stored in role_bindings.role_ref / cluster_role_bindings.role_ref JSONB.
type roleRef struct {
	Kind string `json:"kind"` // "Role" or "ClusterRole"
	Name string `json:"name"`
}

// subject is one entry in the "subjects" JSONB array of a binding.
type subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// AttackPathSummary provides aggregate statistics across all computed paths.
type AttackPathSummary struct {
	TotalPaths      int              `json:"totalPaths"`
	CriticalPaths   int              `json:"criticalPaths"`
	HighPaths       int              `json:"highPaths"`
	MediumPaths     int              `json:"mediumPaths"`
	TargetBreakdown map[string]int   `json:"targetBreakdown"` // target role name → count
	TopPods         []RiskyPod       `json:"topPods"`
}

// BuildAllPaths computes attack paths for all active pods in the cluster.
func (b *RelationalPathBuilder) BuildAllPaths(ctx context.Context, clusterID string) ([]AttackPath, error) {
	// Fetch active pods
	var pods []models.Pod
	q := b.db.WithContext(ctx).Where("deleted_at IS NULL")
	if clusterID != "" {
		q = q.Where("cluster_id = ?", clusterID)
	}
	if err := q.Find(&pods).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch pods: %w", err)
	}

	var allPaths []AttackPath
	for _, pod := range pods {
		paths, err := b.BuildPathsForPod(ctx, pod.UID)
		if err != nil {
			log.Printf("[RelationalPathBuilder] failed to build paths for pod %s: %v", pod.UID, err)
			continue
		}
		allPaths = append(allPaths, paths...)
	}

	return allPaths, nil
}

// BuildPathsForPod computes attack paths originating from a specific pod.
func (b *RelationalPathBuilder) BuildPathsForPod(ctx context.Context, podUID string) ([]AttackPath, error) {
	// 1. Get the pod
	var pod models.Pod
	if err := b.db.WithContext(ctx).
		Where("uid = ? AND deleted_at IS NULL", podUID).
		First(&pod).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []AttackPath{}, nil
		}
		return nil, fmt.Errorf("failed to fetch pod: %w", err)
	}

	// 2. Get the service account
	var sa models.ServiceAccount
	if err := b.db.WithContext(ctx).
		Where("name = ? AND namespace = ? AND cluster_id = ? AND deleted_at IS NULL",
			pod.ServiceAccount, pod.Namespace, pod.ClusterID).
		First(&sa).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Pod has no matching SA in DB; no path
			return []AttackPath{}, nil
		}
		return nil, fmt.Errorf("failed to fetch SA: %w", err)
	}

	// 3. Find RoleBindings referencing this SA
	var roleBindings []models.RoleBinding
	if err := b.db.WithContext(ctx).
		Where("cluster_id = ? AND deleted_at IS NULL", pod.ClusterID).
		Find(&roleBindings).Error; err != nil {
		return nil, err
	}

	// 4. Find ClusterRoleBindings referencing this SA
	var clusterRoleBindings []models.ClusterRoleBinding
	if err := b.db.WithContext(ctx).
		Where("cluster_id = ? AND deleted_at IS NULL", pod.ClusterID).
		Find(&clusterRoleBindings).Error; err != nil {
		return nil, err
	}

	// 5. Load Roles and ClusterRoles
	var roles []models.Role
	if err := b.db.WithContext(ctx).
		Where("cluster_id = ? AND deleted_at IS NULL", pod.ClusterID).
		Find(&roles).Error; err != nil {
		return nil, err
	}

	var clusterRoles []models.ClusterRole
	if err := b.db.WithContext(ctx).
		Where("cluster_id = ? AND deleted_at IS NULL", pod.ClusterID).
		Find(&clusterRoles).Error; err != nil {
		return nil, err
	}

	roleMap := make(map[string]models.Role)
	for _, r := range roles {
		roleMap[r.Namespace+"/"+r.Name] = r
	}
	crMap := make(map[string]models.ClusterRole)
	for _, cr := range clusterRoles {
		crMap[cr.Name] = cr
	}

	// 6. Resolve paths via RoleBindings
	var paths []AttackPath

	for _, rb := range roleBindings {
		if !bindingRefersToSA(rb.Subjects, sa.Name, sa.Namespace) {
			continue
		}

		var ref roleRef
		if err := json.Unmarshal([]byte(rb.RoleRef), &ref); err != nil {
			continue
		}

		var targetName, targetType, targetRules string
		switch ref.Kind {
		case "Role":
			if r, ok := roleMap[rb.Namespace+"/"+ref.Name]; ok {
				targetName = r.Name
				targetType = "Role"
				targetRules = r.Rules
			}
		case "ClusterRole":
			if cr, ok := crMap[ref.Name]; ok {
				targetName = cr.Name
				targetType = "ClusterRole"
				targetRules = cr.Rules
			}
		default:
			continue
		}

		if targetName == "" {
			continue
		}

		riskLevel := classifyRoleRisk(targetName, targetRules)
		if riskLevel == "none" {
			continue // Skip low-interest paths
		}

		path := buildPath(pod, sa, rb.Name, rb.Namespace, "RoleBinding", targetName, targetType, targetRules, riskLevel)
		paths = append(paths, path)
	}

	for _, crb := range clusterRoleBindings {
		if !bindingRefersToSA(crb.Subjects, sa.Name, sa.Namespace) {
			continue
		}

		var ref roleRef
		if err := json.Unmarshal([]byte(crb.RoleRef), &ref); err != nil {
			continue
		}

		if ref.Kind != "ClusterRole" {
			continue
		}

		cr, ok := crMap[ref.Name]
		if !ok {
			continue
		}

		riskLevel := classifyRoleRisk(cr.Name, cr.Rules)
		if riskLevel == "none" {
			continue
		}

		path := buildPath(pod, sa, crb.Name, "", "ClusterRoleBinding", cr.Name, "ClusterRole", cr.Rules, riskLevel)
		paths = append(paths, path)
	}

	return paths, nil
}

// GetSummary computes summary statistics for attack paths.
func (b *RelationalPathBuilder) GetSummary(ctx context.Context, clusterID string) (*AttackPathSummary, error) {
	paths, err := b.BuildAllPaths(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	summary := &AttackPathSummary{
		TotalPaths:      len(paths),
		TargetBreakdown: map[string]int{},
	}

	podRiskMap := map[string]float64{}
	podInfoMap := map[string]RiskyPod{}

	for _, p := range paths {
		switch {
		case p.TotalRisk >= 9.0:
			summary.CriticalPaths++
		case p.TotalRisk >= 7.0:
			summary.HighPaths++
		default:
			summary.MediumPaths++
		}

		// Target = last node
		if len(p.Nodes) > 0 {
			target := p.Nodes[len(p.Nodes)-1]
			name, _ := target.Properties["name"].(string)
			if name != "" {
				summary.TargetBreakdown[name]++
			}
		}

		// Source pod = first node
		if len(p.Nodes) > 0 {
			src := p.Nodes[0]
			uid := src.ID
			if p.TotalRisk > podRiskMap[uid] {
				podRiskMap[uid] = p.TotalRisk
				name, _ := src.Properties["name"].(string)
				ns, _ := src.Properties["namespace"].(string)
				saName, _ := src.Properties["serviceAccount"].(string)
				podInfoMap[uid] = RiskyPod{
					UID:                uid,
					Name:               name,
					Namespace:          ns,
					ServiceAccountName: saName,
					RiskScore:          p.TotalRisk,
					RiskReason:         p.Description,
				}
			}
		}
	}

	// Collect top 10 pods
	for _, rp := range podInfoMap {
		summary.TopPods = append(summary.TopPods, rp)
	}
	// Sort by risk descending (simple insertion since count is small)
	for i := 0; i < len(summary.TopPods); i++ {
		for j := i + 1; j < len(summary.TopPods); j++ {
			if summary.TopPods[j].RiskScore > summary.TopPods[i].RiskScore {
				summary.TopPods[i], summary.TopPods[j] = summary.TopPods[j], summary.TopPods[i]
			}
		}
	}
	if len(summary.TopPods) > 10 {
		summary.TopPods = summary.TopPods[:10]
	}

	return summary, nil
}

// BuildGraphData returns nodes and links for D3 visualization from all computed paths.
func (b *RelationalPathBuilder) BuildGraphData(ctx context.Context, clusterID string) (map[string]interface{}, error) {
	paths, err := b.BuildAllPaths(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	// Deduplicate nodes and links
	nodeSet := map[string]map[string]interface{}{}
	linkSet := map[string]map[string]interface{}{}

	for _, p := range paths {
		for _, n := range p.Nodes {
			nodeSet[n.ID] = map[string]interface{}{
				"id":    n.ID,
				"label": n.Properties["name"],
				"type":  strings.ToLower(n.Type),
				"risk":  classifyRiskLabel(p.TotalRisk),
			}
			// Merge extra properties
			for k, v := range n.Properties {
				nodeSet[n.ID][k] = v
			}
		}
		for _, e := range p.Edges {
			key := e.Source + "->" + e.Target + ":" + e.Type
			linkSet[key] = map[string]interface{}{
				"source": e.Source,
				"target": e.Target,
				"type":   e.Type,
				"value":  p.TotalRisk,
			}
		}
	}

	nodes := make([]map[string]interface{}, 0, len(nodeSet))
	for _, n := range nodeSet {
		nodes = append(nodes, n)
	}

	links := make([]map[string]interface{}, 0, len(linkSet))
	for _, l := range linkSet {
		links = append(links, l)
	}

	return map[string]interface{}{
		"nodes": nodes,
		"links": links,
	}, nil
}

// ──────────────────── helpers ────────────────────

func bindingRefersToSA(subjectsJSON, saName, saNamespace string) bool {
	if subjectsJSON == "" || subjectsJSON == "null" {
		return false
	}
	var subjects []subject
	if err := json.Unmarshal([]byte(subjectsJSON), &subjects); err != nil {
		return false
	}
	for _, s := range subjects {
		if s.Kind == "ServiceAccount" && s.Name == saName {
			if s.Namespace == "" || s.Namespace == saNamespace {
				return true
			}
		}
	}
	return false
}

// classifyRoleRisk returns "critical", "high", "medium", or "none".
func classifyRoleRisk(roleName, rulesJSON string) string {
	lower := strings.ToLower(roleName)
	if lower == "cluster-admin" || strings.Contains(lower, "cluster-admin") {
		return "critical"
	}
	if strings.Contains(lower, "admin") {
		return "high"
	}

	// Parse rules to check for wildcard permissions
	if rulesJSON == "" || rulesJSON == "null" {
		return "none"
	}

	var rules []map[string]interface{}
	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		return "none"
	}

	for _, rule := range rules {
		verbs := toStringSlice(rule["verbs"])
		resources := toStringSlice(rule["resources"])
		apiGroups := toStringSlice(rule["apiGroups"])

		hasWildcardVerb := contains(verbs, "*")
		hasWildcardResource := contains(resources, "*")
		hasWildcardAPI := contains(apiGroups, "*") || contains(apiGroups, "")

		if hasWildcardVerb && hasWildcardResource {
			return "critical"
		}

		// Dangerous verbs on sensitive resources
		dangerousVerbs := hasWildcardVerb || contains(verbs, "create") || contains(verbs, "update") || contains(verbs, "patch") || contains(verbs, "delete")
		sensitiveResources := contains(resources, "secrets") || contains(resources, "pods") || contains(resources, "deployments") || contains(resources, "daemonsets") || contains(resources, "clusterroles") || contains(resources, "clusterrolebindings")

		// Privilege escalation verbs
		if contains(verbs, "escalate") || contains(verbs, "bind") || contains(verbs, "impersonate") {
			return "critical"
		}

		if dangerousVerbs && sensitiveResources && hasWildcardAPI {
			return "high"
		}

		if dangerousVerbs && sensitiveResources {
			return "medium"
		}
	}

	return "none"
}

func buildPath(pod models.Pod, sa models.ServiceAccount, bindingName, bindingNS, bindingType, targetName, targetType, targetRules, riskLevel string) AttackPath {
	totalRisk := riskToScore(riskLevel)
	difficulty := difficultyFromRiskLevel(riskLevel)
	impact := impactFromRiskLevel(riskLevel)

	podNode := PathNode{
		ID:   pod.UID,
		Type: "Pod",
		Properties: map[string]interface{}{
			"name":           pod.Name,
			"namespace":      pod.Namespace,
			"serviceAccount": pod.ServiceAccount,
			"hostNetwork":    pod.HostNetwork,
			"hostPID":        pod.HostPID,
		},
	}

	saNode := PathNode{
		ID:   sa.UID,
		Type: "ServiceAccount",
		Properties: map[string]interface{}{
			"name":      sa.Name,
			"namespace": sa.Namespace,
		},
	}

	bindingID := fmt.Sprintf("binding:%s/%s", bindingNS, bindingName)
	bindingNode := PathNode{
		ID:   bindingID,
		Type: bindingType,
		Properties: map[string]interface{}{
			"name":      bindingName,
			"namespace": bindingNS,
		},
	}

	targetID := fmt.Sprintf("role:%s", targetName)
	targetNode := PathNode{
		ID:   targetID,
		Type: targetType,
		Properties: map[string]interface{}{
			"name":      targetName,
			"riskLevel": riskLevel,
		},
	}

	return AttackPath{
		Nodes: []PathNode{podNode, saNode, bindingNode, targetNode},
		Edges: []PathEdge{
			{Type: "USES_SERVICE_ACCOUNT", Source: pod.UID, Target: sa.UID},
			{Type: "REFERENCED_BY", Source: sa.UID, Target: bindingID},
			{Type: "GRANTS_ROLE", Source: bindingID, Target: targetID},
		},
		TotalRisk:   totalRisk,
		Difficulty:  difficulty,
		Impact:      impact,
		Length:      3,
		Description: describeAttackPath(pod.Name, sa.Name, targetName, riskLevel),
	}
}

func riskToScore(level string) float64 {
	switch level {
	case "critical":
		return 10.0
	case "high":
		return 8.0
	case "medium":
		return 5.5
	default:
		return 3.0
	}
}

func difficultyFromRiskLevel(level string) float64 {
	switch level {
	case "critical":
		return 0.2 // Easy to exploit
	case "high":
		return 0.4
	case "medium":
		return 0.6
	default:
		return 0.8
	}
}

func impactFromRiskLevel(level string) float64 {
	switch level {
	case "critical":
		return 1.0 // Full cluster compromise
	case "high":
		return 0.8
	case "medium":
		return 0.5
	default:
		return 0.3
	}
}

func describeAttackPath(podName, saName, roleName, riskLevel string) string {
	switch riskLevel {
	case "critical":
		return fmt.Sprintf("Pod '%s' → SA '%s' → cluster-admin role '%s' (full cluster access)", podName, saName, roleName)
	case "high":
		return fmt.Sprintf("Pod '%s' → SA '%s' → admin role '%s' (elevated privileges)", podName, saName, roleName)
	default:
		return fmt.Sprintf("Pod '%s' → SA '%s' → role '%s' (sensitive resource access)", podName, saName, roleName)
	}
}

func classifyRiskLabel(score float64) string {
	if score >= 9.0 {
		return "critical"
	}
	if score >= 7.0 {
		return "high"
	}
	if score >= 4.0 {
		return "medium"
	}
	return "low"
}

func toStringSlice(val interface{}) []string {
	if val == nil {
		return nil
	}
	arr, ok := val.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func contains(s []string, v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}
