package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/rbac"
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
	TotalPaths      int            `json:"totalPaths"`
	CriticalPaths   int            `json:"criticalPaths"`
	HighPaths       int            `json:"highPaths"`
	MediumPaths     int            `json:"mediumPaths"`
	TargetBreakdown map[string]int `json:"targetBreakdown"` // target role name → count
	TopPods         []RiskyPod     `json:"topPods"`
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
// Phase 1.2: paths are enriched with pod_capabilities (escape edges) and
// pod_attack_steps (active step edges) from the PCE pipeline.
// Phase 1.3: computed paths are persisted to the attack_paths table via upsert.
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

	// Phase 1.2 — Load PCE capability facts for this pod
	var podCaps []models.PodCapability
	if b.db.Migrator().HasTable("pod_capabilities") {
		if err := b.db.WithContext(ctx).
			Where("pod_uid = ?", podUID).
			Find(&podCaps).Error; err != nil {
			log.Printf("[RelationalPathBuilder] warning: failed to load pod_capabilities for %s: %v", podUID, err)
		}
	}

	// Phase 1.2 — Load PCE attack steps for this pod
	var podAttackSteps []models.PodAttackStep
	if b.db.Migrator().HasTable("pod_attack_steps") {
		if err := b.db.WithContext(ctx).
			Where("pod_uid = ?", podUID).
			Find(&podAttackSteps).Error; err != nil {
			log.Printf("[RelationalPathBuilder] warning: failed to load pod_attack_steps for %s: %v", podUID, err)
		}
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

		riskLevel := rbac.ClassifyRoleRisk(targetName, targetRules)
		if riskLevel == "none" {
			continue // Skip low-interest paths
		}

		path := buildPath(pod, sa, rb.Name, rb.Namespace, "RoleBinding", targetName, targetType, targetRules, riskLevel)
		// Phase 1.2: enrich with PCE facts
		enrichPathWithPCE(&path, podCaps, podAttackSteps)
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

		riskLevel := rbac.ClassifyRoleRisk(cr.Name, cr.Rules)
		if riskLevel == "none" {
			continue
		}

		path := buildPath(pod, sa, crb.Name, "", "ClusterRoleBinding", cr.Name, "ClusterRole", cr.Rules, riskLevel)
		// Phase 1.2: enrich with PCE facts
		enrichPathWithPCE(&path, podCaps, podAttackSteps)
		paths = append(paths, path)
	}

	// Phase 1.3: persist computed paths (upsert by pod_uid + path_id)
	if err := persistAttackPaths(ctx, b.db, podUID, paths); err != nil {
		log.Printf("[RelationalPathBuilder] warning: failed to persist attack paths for pod %s: %v", podUID, err)
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

func buildPath(pod models.Pod, sa models.ServiceAccount, bindingName, bindingNS, bindingType, targetName, targetType, targetRules, riskLevel string) AttackPath {
	totalRisk := riskToScore(riskLevel)
	difficulty := difficultyFromRiskLevel(riskLevel)
	impact := impactFromRiskLevel(riskLevel)

	// Boost risk based on pod security posture
	posture := podSecurityPostureBoost(pod)
	totalRisk = clampFloat(totalRisk+posture, 0, 10.0)
	if posture > 0 {
		difficulty = clampFloat(difficulty-0.1, 0.1, 1.0)
	}

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

// enrichPathWithPCE adds PCE capability edges and attack-step edges to a path,
// and boosts TotalRisk / reduces Difficulty based on capability state.
// This implements Phase 1.2 of the Unified Risk Pipeline (PCE-1 gap closure).
func enrichPathWithPCE(path *AttackPath, caps []models.PodCapability, steps []models.PodAttackStep) {
	if len(caps) == 0 && len(steps) == 0 {
		return
	}

	const (
		capESCPrivPod      = "ESC_PRIV_POD"
		capESCHostPath     = "ESC_HOSTPATH_NODE"
		capESCRuntimeActive = "ESC_RUNTIME_ACTIVE"
	)

	escapeCapIDs := map[string]struct{}{
		capESCPrivPod:       {},
		capESCHostPath:      {},
		capESCRuntimeActive: {},
	}

	enriched := false
	podNodeID := ""
	if len(path.Nodes) > 0 {
		podNodeID = path.Nodes[0].ID
	}

	for _, cap := range caps {
		if _, isEscape := escapeCapIDs[cap.CapabilityID]; !isEscape {
			continue
		}

		// Add a capability node and escape edge
		capNodeID := fmt.Sprintf("cap:%s:%s", cap.PodUID, cap.CapabilityID)
		capNode := PathNode{
			ID:   capNodeID,
			Type: "Capability",
			Properties: map[string]interface{}{
				"capabilityId": cap.CapabilityID,
				"severity":     cap.Severity,
				"state":        cap.State,
			},
		}
		path.Nodes = append(path.Nodes, capNode)
		if podNodeID != "" {
			path.Edges = append(path.Edges, PathEdge{
				Type:   "HAS_CAPABILITY",
				Source: podNodeID,
				Target: capNodeID,
			})
		}

		// Boost path risk based on capability state
		switch cap.State {
		case "confirmed":
			path.TotalRisk = clampFloat(path.TotalRisk+1.0, 0, 10.0)
			path.Difficulty = clampFloat(path.Difficulty-0.1, 0.1, 1.0)
		case "exploited":
			path.TotalRisk = clampFloat(path.TotalRisk+2.5, 0, 10.0)
			path.Difficulty = clampFloat(path.Difficulty-0.3, 0.1, 1.0)
		}

		enriched = true
	}

	// Add edges for active attack steps
	for _, step := range steps {
		stepNodeID := fmt.Sprintf("step:%s:%s", step.PodUID, step.StepID)
		stepNode := PathNode{
			ID:   stepNodeID,
			Type: "AttackStep",
			Properties: map[string]interface{}{
				"stepId":     step.StepID,
				"category":   step.Category,
				"confidence": step.Confidence,
			},
		}
		path.Nodes = append(path.Nodes, stepNode)
		if podNodeID != "" {
			edgeType := "HAS_ATTACK_STEP"
			if step.StepID == "NODE_CRED_DUMP" {
				edgeType = "CAN_STEAL_CREDENTIALS"
			}
			path.Edges = append(path.Edges, PathEdge{
				Type:   edgeType,
				Source: podNodeID,
				Target: stepNodeID,
			})
		}
		enriched = true
	}

	if enriched {
		path.EnrichedFromPCE = true
		path.Length = len(path.Nodes) - 1
	}
}

// persistAttackPaths upserts computed attack paths into the attack_paths table.
// If the table does not yet exist (pre-migration environment), this is a no-op.
func persistAttackPaths(ctx context.Context, db *gorm.DB, podUID string, paths []AttackPath) error {
	if !db.Migrator().HasTable("attack_paths") {
		return nil
	}

	for i, p := range paths {
		nodesJSON, _ := json.Marshal(p.Nodes)
		edgesJSON, _ := json.Marshal(p.Edges)

		// Stable path ID: based on position so re-computation produces the same key.
		pathID := fmt.Sprintf("%s-path-%d", podUID, i)

		record := models.AttackPath{
			PodUID:          podUID,
			PathID:          pathID,
			Nodes:           string(nodesJSON),
			Edges:           string(edgesJSON),
			TotalRisk:       p.TotalRisk,
			Difficulty:      p.Difficulty,
			Impact:          p.Impact,
			Length:          p.Length,
			Description:     p.Description,
			EnrichedFromPCE: p.EnrichedFromPCE,
		}

		if err := db.WithContext(ctx).
			Where("pod_uid = ? AND path_id = ?", podUID, pathID).
			Assign(record).
			FirstOrCreate(&record).Error; err != nil {
			return fmt.Errorf("upsert path %s: %w", pathID, err)
		}
	}
	return nil
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

func containsAny(s []string, vals ...string) bool {
	for _, item := range s {
		for _, v := range vals {
			if item == v {
				return true
			}
		}
	}
	return false
}

// podSecurityPostureBoost returns an additive risk score boost (0–1.5)
// based on how privileged the pod's security posture is.
// Pods with hostNetwork, hostPID, hostIPC, or disabled SA token automount
// have an easier lateral-movement surface, so their attack paths are riskier.
func podSecurityPostureBoost(pod models.Pod) float64 {
	var boost float64
	if pod.HostNetwork {
		boost += 0.5
	}
	if pod.HostPID {
		boost += 0.5
	}
	if pod.HostIPC {
		boost += 0.3
	}
	// automountServiceAccountToken defaults to true; if explicitly false the
	// token isn't mounted so the SA path is harder — no boost (handled implicitly).
	if boost > 1.5 {
		boost = 1.5
	}
	return boost
}

// clampFloat clamps v to [lo, hi].
func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
