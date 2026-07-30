package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

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

// AttackPathsViewBundle is a single round-trip payload for the attack-paths dashboard
// (graph + summary + chains + objectives) built from one BuildAllPaths pass.
type AttackPathsViewBundle struct {
	Graph      map[string]interface{} `json:"graph"`
	Summary    *AttackPathSummary     `json:"summary"`
	Chains     []AttackChain          `json:"chains"`
	Objectives []AttackObjective      `json:"objectives"`
	// Paths is the full primitive path list (same ordering as path_id p0..p{n-1} used in chains).
	Paths []AttackPath `json:"paths"`
}

type ReachabilityDecision int

const (
	ReachabilityDeny ReachabilityDecision = iota
	ReachabilityAllow
	ReachabilitySoftAllow
)

type ReachabilityContext struct {
	ObservedEgressIPs map[string]time.Time
	DenyCache         map[string]time.Time
	Now               time.Time
	FallbackTTL       time.Duration
	DenyTTL           time.Duration
}

// BuildAllPaths computes attack paths for all active pods in the cluster (or all clusters when clusterID is empty).
// persist=true writes attack_paths rows and clears the read cache; use only from background reconcile / explicit rebuild.
// Concurrent callers with the same clusterID and persist flag share one compute via singleflight until the TTL cache fills.
func (b *RelationalPathBuilder) BuildAllPaths(ctx context.Context, clusterID string, persist bool) ([]AttackPath, error) {
	if persist {
		clearAttackPathBuildCache()
	} else if cached, ok := getCachedAttackPathsAll(clusterID); ok {
		return cached, nil
	}

	key := buildAllPathsSingleflightKey(clusterID, persist)
	v, err, _ := attackPathBuildFlight.Do(key, func() (interface{}, error) {
		return b.buildAllPathsNoCache(ctx, clusterID, persist)
	})
	if err != nil {
		return nil, err
	}
	return v.([]AttackPath), nil
}

func (b *RelationalPathBuilder) buildAllPathsNoCache(ctx context.Context, clusterID string, persist bool) ([]AttackPath, error) {
	var pods []models.Pod
	q := b.db.WithContext(ctx).Where("deleted_at IS NULL")
	if clusterID != "" {
		q = q.Where("cluster_id = ?", clusterID)
	}
	if err := q.Find(&pods).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch pods: %w", err)
	}

	byCluster := make(map[string][]models.Pod)
	for _, p := range pods {
		if strings.TrimSpace(p.ClusterID) == "" {
			continue
		}
		cid := p.ClusterID
		byCluster[cid] = append(byCluster[cid], p)
	}

	var allPaths []AttackPath
	for cid, plist := range byCluster {
		snap, err := b.loadClusterPathSnapshot(ctx, cid)
		if err != nil {
			log.Printf("[RelationalPathBuilder] cluster %s snapshot failed: %v", cid, err)
			continue
		}
		for i := range plist {
			pod := plist[i]
			paths, err := b.buildPathsForPodWithSnapshot(ctx, &pod, snap, persist)
			if err != nil {
				log.Printf("[RelationalPathBuilder] failed to build paths for pod %s: %v", pod.UID, err)
				continue
			}
			allPaths = append(allPaths, paths...)
		}
	}

	if persist {
		if err := cleanupStaleAttackPaths(ctx, b.db, clusterID); err != nil {
			log.Printf("[RelationalPathBuilder] stale attack_paths cleanup failed: %v", err)
		}
	}

	if !persist {
		setCachedAttackPathsAll(clusterID, allPaths)
	}
	return allPaths, nil
}

// clusterPathSnapshot holds cluster-wide RBAC + workload rows loaded once per BuildAllPaths pass.
type clusterPathSnapshot struct {
	ClusterID           string
	SaByNSName          map[string]models.ServiceAccount
	RoleBindings        []models.RoleBinding
	ClusterRoleBindings []models.ClusterRoleBinding
	RoleMap             map[string]models.Role
	CrMap               map[string]models.ClusterRole
	ClusterPods         []models.Pod
	DenyCache           map[string]time.Time
	// HardeningHints captures cluster-level security signals derived from available
	// pod and RBAC data. Used by computeChainConfidenceWithHints (FIX I).
	HardeningHints ClusterHardeningHints
}

func (b *RelationalPathBuilder) loadClusterPathSnapshot(ctx context.Context, clusterID string) (*clusterPathSnapshot, error) {
	clusterID = strings.TrimSpace(clusterID)
	applyClusterScope := func(q *gorm.DB) *gorm.DB {
		if clusterID == "" {
			return q
		}
		return q.Where("cluster_id = ?", clusterID)
	}

	var serviceAccounts []models.ServiceAccount
	if err := applyClusterScope(b.db.WithContext(ctx).Where("deleted_at IS NULL")).
		Find(&serviceAccounts).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch service accounts: %w", err)
	}
	saByNSName := make(map[string]models.ServiceAccount, len(serviceAccounts))
	for _, cur := range serviceAccounts {
		saByNSName[cur.Namespace+"/"+cur.Name] = cur
	}

	var roleBindings []models.RoleBinding
	if err := applyClusterScope(b.db.WithContext(ctx).Where("deleted_at IS NULL")).
		Find(&roleBindings).Error; err != nil {
		return nil, err
	}
	var clusterRoleBindings []models.ClusterRoleBinding
	if err := applyClusterScope(b.db.WithContext(ctx).Where("deleted_at IS NULL")).
		Find(&clusterRoleBindings).Error; err != nil {
		return nil, err
	}
	var roles []models.Role
	if err := applyClusterScope(b.db.WithContext(ctx).Where("deleted_at IS NULL")).
		Find(&roles).Error; err != nil {
		return nil, err
	}
	var clusterRoles []models.ClusterRole
	if err := applyClusterScope(b.db.WithContext(ctx).Where("deleted_at IS NULL")).
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

	var clusterPods []models.Pod
	if err := applyClusterScope(b.db.WithContext(ctx).Where("deleted_at IS NULL")).
		Find(&clusterPods).Error; err != nil {
		return nil, err
	}

	return &clusterPathSnapshot{
		ClusterID:           clusterID,
		SaByNSName:          saByNSName,
		RoleBindings:        roleBindings,
		ClusterRoleBindings: clusterRoleBindings,
		RoleMap:             roleMap,
		CrMap:               crMap,
		ClusterPods:         clusterPods,
		DenyCache:           b.loadDenyCache(ctx),
		HardeningHints:      deriveHardeningHints(clusterPods),
	}, nil
}

// deriveHardeningHints builds a ClusterHardeningHints from available pod-level
// data in the snapshot. We cannot query kubelet auth config directly, so we
// use observable signals as proxies:
//
//	AutomountDefault: true when the majority of pods mount SA tokens.
//	ProjectedTokensOnly: set when ALL pods with automount=true also have
//	  PodSecurityContext that indicates a projected volume workload style.
//	PodSecurityLevel: inferred from pod Security Context if runAsNonRoot=true
//	  and allowPrivilegeEscalation=false across the cluster (baseline heuristic).
func deriveHardeningHints(pods []models.Pod) ClusterHardeningHints {
	hints := DefaultHardeningHints()
	if len(pods) == 0 {
		return hints
	}

	mountCount := 0
	noMountCount := 0
	privilegedCount := 0

	for _, p := range pods {
		if p.AutomountServiceAccountToken != nil {
			if *p.AutomountServiceAccountToken {
				mountCount++
			} else {
				noMountCount++
			}
		} else {
			// nil = default = true in Kubernetes
			mountCount++
		}
		if p.HostPID || p.HostNetwork || p.HostIPC {
			privilegedCount++
		}
	}

	// AutomountDefault: true when >50% of pods mount tokens.
	hints.AutomountDefault = mountCount >= noMountCount

	// ProjectedTokensOnly: inferred when most pods opt out of automount
	// (suggests projected service account token volumes are used instead).
	if noMountCount > mountCount && len(pods) >= 3 {
		hints.ProjectedTokensOnly = true
	}

	// PodSecurityLevel: heuristic from privileged workload ratio.
	// >20% privileged → "privileged", 0% → "restricted" (rough proxy).
	if privilegedCount == 0 && len(pods) >= 3 {
		hints.PodSecurityLevel = "baseline" // can't confirm restricted without PSA labels
	} else if float64(privilegedCount)/float64(len(pods)) > 0.20 {
		hints.PodSecurityLevel = "privileged"
	}

	return hints
}

// BuildPathsForPod computes attack paths originating from a specific pod.
// Phase 1.2: paths are enriched with pod_capabilities (escape edges) and
// pod_attack_steps (active step edges) from the PCE pipeline.
// Phase 1.3: when persist=true, paths are written to attack_paths (background reconcile / explicit rebuild only).
func (b *RelationalPathBuilder) BuildPathsForPod(ctx context.Context, podUID string, persist bool) ([]AttackPath, error) {
	var pod models.Pod
	if err := b.db.WithContext(ctx).
		Where("uid = ? AND deleted_at IS NULL", podUID).
		First(&pod).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return []AttackPath{}, nil
		}
		return nil, fmt.Errorf("failed to fetch pod: %w", err)
	}
	snap, err := b.loadClusterPathSnapshot(ctx, pod.ClusterID)
	if err != nil {
		return nil, err
	}
	return b.buildPathsForPodWithSnapshot(ctx, &pod, snap, persist)
}

func (b *RelationalPathBuilder) buildPathsForPodWithSnapshot(ctx context.Context, pod *models.Pod, snap *clusterPathSnapshot, persist bool) ([]AttackPath, error) {
	podUID := pod.UID
	if _, ok := snap.SaByNSName[pod.Namespace+"/"+pod.ServiceAccount]; !ok {
		return []AttackPath{}, nil
	}
	sa := snap.SaByNSName[pod.Namespace+"/"+pod.ServiceAccount]

	observedDestIPs := map[string]time.Time{}
	if b.db.Migrator().HasTable(&models.PodNetworkConnection{}) {
		var conns []models.PodNetworkConnection
		since := time.Now().Add(-attackPathObservedEgressWindowFromEnv())
		if err := b.db.WithContext(ctx).
			Where("pod_uid = ? AND observed_at >= ?", podUID, since).
			Find(&conns).Error; err == nil {
			for _, c := range conns {
				if strings.TrimSpace(c.DestIP) != "" {
					ip := strings.TrimSpace(c.DestIP)
					if prev, ok := observedDestIPs[ip]; !ok || c.ObservedAt.After(prev) {
						observedDestIPs[ip] = c.ObservedAt
					}
				}
			}
		}
	}
	reachCtx := ReachabilityContext{
		ObservedEgressIPs: observedDestIPs,
		DenyCache:         snap.DenyCache,
		Now:               time.Now(),
		FallbackTTL:       attackPathFallbackTTLFromEnv(),
		DenyTTL:           attackPathDenyTTLFromEnv(),
	}

	var podCaps []models.PodCapability
	if b.db.Migrator().HasTable("pod_capabilities") {
		if err := b.db.WithContext(ctx).
			Where("pod_uid = ?", podUID).
			Find(&podCaps).Error; err != nil {
			log.Printf("[RelationalPathBuilder] warning: failed to load pod_capabilities for %s: %v", podUID, err)
		}
	}

	var podAttackSteps []models.PodAttackStep
	if b.db.Migrator().HasTable("pod_attack_steps") {
		if err := b.db.WithContext(ctx).
			Where("pod_uid = ?", podUID).
			Find(&podAttackSteps).Error; err != nil {
			log.Printf("[RelationalPathBuilder] warning: failed to load pod_attack_steps for %s: %v", podUID, err)
		}
	}

	var paths []AttackPath
	if isServiceAccountPathFeasible(*pod, podCaps) {
		criticalCVE := b.hasCriticalCVEInsight(ctx, podUID)
		highRiskSource := pod.HostNetwork || hasEscapeCapability(podCaps) || criticalCVE
		paths = buildDeterministicPaths(
			*pod, sa,
			snap.ClusterPods,
			snap.SaByNSName,
			snap.RoleBindings, snap.ClusterRoleBindings,
			snap.RoleMap, snap.CrMap,
			podCaps, podAttackSteps,
			reachCtx,
			highRiskSource,
		)
	}

	if persist {
		if err := persistAttackPaths(ctx, b.db, podUID, paths); err != nil {
			log.Printf("[RelationalPathBuilder] warning: failed to persist attack paths for pod %s: %v", podUID, err)
		}
	}

	return paths, nil
}

func isServiceAccountPathFeasible(pod models.Pod, podCaps []models.PodCapability) bool {
	if pod.AutomountServiceAccountToken != nil && !*pod.AutomountServiceAccountToken {
		for _, c := range podCaps {
			if isEdgeCapability(c.CapabilityID) {
				return true
			}
		}
		return false
	}
	return true
}

func hasEscapeCapability(caps []models.PodCapability) bool {
	for _, c := range caps {
		if c.CapabilityID == "ESC_PRIV_POD" || c.CapabilityID == "ESC_HOSTPATH_NODE" || c.CapabilityID == "ESC_RUNTIME_ACTIVE" {
			return true
		}
	}
	return false
}

func (b *RelationalPathBuilder) hasCriticalCVEInsight(ctx context.Context, podUID string) bool {
	if !b.db.Migrator().HasTable("insights") {
		return false
	}
	var count int64
	_ = b.db.WithContext(ctx).Model(&models.Insight{}).
		Where("resource_uid = ? AND deleted_at IS NULL AND status = 'active' AND lower(insight_type) = 'vulnerability' AND lower(severity) = 'critical'", podUID).
		Count(&count).Error
	return count > 0
}

func buildDeterministicPaths(
	pod models.Pod,
	sa models.ServiceAccount,
	clusterPods []models.Pod,
	saByNSName map[string]models.ServiceAccount,
	roleBindings []models.RoleBinding,
	clusterRoleBindings []models.ClusterRoleBinding,
	roleMap map[string]models.Role,
	crMap map[string]models.ClusterRole,
	podCaps []models.PodCapability,
	podAttackSteps []models.PodAttackStep,
	reachCtx ReachabilityContext,
	highRiskSource bool,
) []AttackPath {
	g := NewAttackGraph()
	podLabel := strings.TrimSpace(pod.Name)
	if podLabel == "" {
		podLabel = pod.UID
	}
	g.AddNode(GraphNode{ID: pod.UID, Type: NodeTypePod, Namespace: pod.Namespace, Label: podLabel})
	g.AddNode(GraphNode{ID: sa.UID, Type: NodeTypeServiceAccount, Namespace: sa.Namespace, Label: sa.Name})
	g.AddEdge(GraphEdge{From: pod.UID, To: sa.UID, Type: EdgeTypeServiceAccount, Exploitability: 0.9})

	addRBACChainForSA(g, sa, roleBindings, clusterRoleBindings, roleMap, crMap)

	for _, peer := range clusterPods {
		if peer.UID == pod.UID {
			continue
		}
		decision := isNetworkReachable(pod, peer, reachCtx)
		if decision == ReachabilityDeny {
			continue
		}
		edgeExploitAllow := 0.6
		edgeExploit := edgeExploitAllow
		edgeType := EdgeTypeNetworkReach
		if decision == ReachabilitySoftAllow {
			edgeExploit = edgeExploitAllow * 0.6
			edgeType = EdgeTypeNetworkReachSoft
		}
		peerLabel := strings.TrimSpace(peer.Name)
		if peerLabel == "" {
			peerLabel = peer.UID
		}
		g.AddNode(GraphNode{ID: peer.UID, Type: NodeTypePod, Namespace: peer.Namespace, Label: peerLabel})
		g.AddEdge(GraphEdge{From: pod.UID, To: peer.UID, Type: edgeType, Exploitability: edgeExploit})
		peerSA, ok := saByNSName[peer.Namespace+"/"+peer.ServiceAccount]
		if !ok {
			continue
		}
		g.AddNode(GraphNode{ID: peerSA.UID, Type: NodeTypeServiceAccount, Namespace: peerSA.Namespace, Label: peerSA.Name})
		g.AddEdge(GraphEdge{From: peer.UID, To: peerSA.UID, Type: EdgeTypeServiceAccount, Exploitability: 0.75})
		addRBACChainForSA(g, peerSA, roleBindings, clusterRoleBindings, roleMap, crMap)
	}

	nodeID := ""
	if pod.NodeName != "" {
		nodeID = "node:" + pod.NodeName
	}
	stateAmp := capabilityStateAmplifier(pod, podCaps, len(reachCtx.ObservedEgressIPs) > 0)
	for _, c := range podCaps {
		if !isEdgeCapability(c.CapabilityID) {
			continue
		}
		capNodeID := fmt.Sprintf("cap:%s:%s", c.PodUID, c.CapabilityID)
		g.AddNode(GraphNode{ID: capNodeID, Type: NodeTypeCapability, Namespace: pod.Namespace, Label: c.CapabilityID})
		edgeType := EdgeTypeContainerEscape
		exploit := 0.55
		if c.CapabilityID == "ESC_HOSTPATH_NODE" {
			edgeType = EdgeTypeHostAccess
			exploit = 0.75
		}
		if c.State == "exploited" || c.State == "chained" {
			exploit += 0.15
		}
		exploit += stateAmp
		g.AddEdge(GraphEdge{From: pod.UID, To: capNodeID, Type: edgeType, Exploitability: exploit})
		if nodeID != "" && (c.CapabilityID == "ESC_HOSTPATH_NODE" || c.CapabilityID == "ESC_RUNTIME_ACTIVE") {
			g.AddNode(GraphNode{ID: nodeID, Type: NodeTypeNode, Namespace: "kube-system", Label: pod.NodeName})
			g.AddEdge(GraphEdge{From: capNodeID, To: nodeID, Type: EdgeTypeLateralMove, Exploitability: clampFloat(0.8+stateAmp, 0.1, 0.99)})
		}
	}

	for _, step := range podAttackSteps {
		stepID := fmt.Sprintf("step:%s:%s", step.PodUID, step.StepID)
		g.AddNode(GraphNode{ID: stepID, Type: NodeTypeAttackStep, Namespace: pod.Namespace, Label: step.StepID})
		edgeType := EdgeTypeHasAttackStep
		if step.StepID == "NODE_CRED_DUMP" {
			edgeType = EdgeTypeCanStealCredential
		}
		g.AddEdge(GraphEdge{From: pod.UID, To: stepID, Type: edgeType, Exploitability: clampFloat(step.Confidence, 0.2, 0.95)})
	}

	targetTypes := map[string]bool{
		NodeTypeRole:        true,
		NodeTypeClusterRole: true,
		NodeTypeNode:        true,
	}
	paths := g.BuildTopPathsAdaptive(
		pod.UID,
		targetTypes,
		highRiskSource,
		PathBuildOptions{},
		func(edge GraphEdge, from, to GraphNode) bool {
			if from.Namespace != "" && to.Namespace != "" && from.Namespace != to.Namespace &&
				edge.Type != EdgeTypeRbacBinding && edge.Type != EdgeTypeGrantsRole {
				return false
			}
			return true
		},
	)
	return convertGraphPathsToAttackPaths(paths, g)
}

func addRBACChainForSA(
	g *AttackGraph,
	sa models.ServiceAccount,
	roleBindings []models.RoleBinding,
	clusterRoleBindings []models.ClusterRoleBinding,
	roleMap map[string]models.Role,
	crMap map[string]models.ClusterRole,
) {
	for _, rb := range roleBindings {
		if !bindingRefersToSA(rb.Subjects, sa.Name, sa.Namespace) {
			continue
		}
		var ref roleRef
		if err := json.Unmarshal([]byte(rb.RoleRef), &ref); err != nil {
			continue
		}
		var targetID, targetType, targetName, targetRules string
		switch ref.Kind {
		case "Role":
			if r, ok := roleMap[rb.Namespace+"/"+ref.Name]; ok {
				targetID, targetType, targetName, targetRules = "role:"+r.Name, NodeTypeRole, r.Name, r.Rules
			}
		case "ClusterRole":
			if cr, ok := crMap[ref.Name]; ok {
				targetID, targetType, targetName, targetRules = "role:"+cr.Name, NodeTypeClusterRole, cr.Name, cr.Rules
			}
		}
		if targetID == "" {
			continue
		}
		riskLevel := rbac.ClassifyRoleRisk(targetName, targetRules)
		if riskLevel == "none" {
			continue
		}
		// FIX E: derive semantic capabilities from actual RBAC rules.
		semanticCaps := semanticCapsFromRules(targetRules)
		bindingID := fmt.Sprintf("binding:%s/%s", rb.Namespace, rb.Name)
		g.AddNode(GraphNode{ID: bindingID, Type: NodeTypeRoleBinding, Namespace: rb.Namespace, Label: rb.Name})
		g.AddNode(GraphNode{ID: targetID, Type: targetType, Namespace: rb.Namespace, Label: targetName, SemanticCaps: semanticCaps})
		g.AddEdge(GraphEdge{From: sa.UID, To: bindingID, Type: EdgeTypeRbacBinding, Exploitability: 0.85})
		g.AddEdge(GraphEdge{From: bindingID, To: targetID, Type: EdgeTypeGrantsRole, Exploitability: exploitabilityByRisk(riskLevel)})
	}

	for _, crb := range clusterRoleBindings {
		if !bindingRefersToSA(crb.Subjects, sa.Name, sa.Namespace) {
			continue
		}
		var ref roleRef
		if err := json.Unmarshal([]byte(crb.RoleRef), &ref); err != nil || ref.Kind != "ClusterRole" {
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
		// FIX E: derive semantic capabilities from actual RBAC rules.
		semanticCaps := semanticCapsFromRules(cr.Rules)
		bindingID := fmt.Sprintf("binding:/%s", crb.Name)
		targetID := "role:" + cr.Name
		g.AddNode(GraphNode{ID: bindingID, Type: NodeTypeClusterBinding, Namespace: "", Label: crb.Name})
		g.AddNode(GraphNode{ID: targetID, Type: NodeTypeClusterRole, Namespace: "", Label: cr.Name, SemanticCaps: semanticCaps})
		g.AddEdge(GraphEdge{From: sa.UID, To: bindingID, Type: EdgeTypeRbacBinding, Exploitability: 0.9})
		g.AddEdge(GraphEdge{From: bindingID, To: targetID, Type: EdgeTypeGrantsRole, Exploitability: exploitabilityByRisk(riskLevel)})
	}
}

// semanticCapsFromRules maps RBAC rules JSON to semantic capability strings
// used by deriveProvides and objectiveFromChainContext. These are the same
// tokens as DerivedCapability.Type from rbac.DeriveCapabilities.
func semanticCapsFromRules(rulesJSON string) []string {
	derived := rbac.DeriveCapabilities(rulesJSON)
	if len(derived) == 0 {
		return nil
	}
	out := make([]string, 0, len(derived))
	for _, d := range derived {
		switch d.Type {
		case "SECRET_READ":
			out = append(out, "DATA_ACCESS:secrets")
		case "WORKLOAD_CREATE":
			out = append(out, "WORKLOAD_CONTROL")
		case "EXEC_ACCESS":
			out = append(out, "EXECUTION")
		case "NODE_PROXY":
			out = append(out, "NODE_ACCESS", NodeAccessKubelet)
		case "CERT_ISSUE":
			out = append(out, "IDENTITY_FORGE")
		}
	}
	return out
}

func isNetworkReachable(fromPod, toPod models.Pod, ctx ReachabilityContext) ReachabilityDecision {
	if ctx.Now.IsZero() {
		ctx.Now = time.Now()
	}
	edgeKey := fromPod.UID + "->" + toPod.UID
	if ts, ok := ctx.DenyCache[edgeKey]; ok && !ts.IsZero() && ctx.Now.Sub(ts) < ctx.DenyTTL {
		return ReachabilityDeny
	}
	if ip := strings.TrimSpace(toPod.PodIP); ip != "" {
		if ts, ok := ctx.ObservedEgressIPs[ip]; ok && !ts.IsZero() && ctx.Now.Sub(ts) <= attackPathObservedEgressWindowFromEnv() {
			return ReachabilityAllow
		}
	}
	if isPodWithinFallbackTTL(fromPod, ctx.FallbackTTL) {
		return ReachabilitySoftAllow
	}
	return ReachabilityDeny
}

func attackPathFallbackTTLFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_ATTACK_PATH_FALLBACK_TTL"))
	if raw == "" {
		return 6 * time.Hour
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 6 * time.Hour
	}
	return d
}

func attackPathObservedEgressWindowFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_ATTACK_PATH_OBSERVED_EGRESS_WINDOW"))
	if raw == "" {
		return 24 * time.Hour
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 24 * time.Hour
	}
	return d
}

func attackPathDenyTTLFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_ATTACK_PATH_DENY_TTL"))
	if raw == "" {
		return 15 * time.Minute
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 15 * time.Minute
	}
	return d
}

func isPodWithinFallbackTTL(pod models.Pod, ttl time.Duration) bool {
	if ttl <= 0 || pod.StartTime.Time == nil {
		return false
	}
	// Avoid restart-loop free pass: warm-up fallback is for truly new pods.
	if pod.RestartCount > 0 {
		return false
	}
	if pod.StartTime.Time.IsZero() {
		return false
	}
	effectiveStart := *pod.StartTime.Time
	if !pod.CreatedAt.IsZero() && pod.CreatedAt.Before(effectiveStart) {
		effectiveStart = pod.CreatedAt
	}
	return time.Since(effectiveStart) <= ttl
}

// isEdgeCapability returns true for capability IDs that represent a potential
// container-to-node escape vector, regardless of automount token settings.
// Keep in sync with capability/constants.go escape capability definitions.
func isEdgeCapability(capabilityID string) bool {
	switch strings.ToUpper(strings.TrimSpace(capabilityID)) {
	case "ESC_HOSTPATH_NODE",
		"ESC_RUNTIME_ACTIVE",
		"ESC_PRIV_POD",          // privileged container
		"ESC_HOSTPID_POD",       // hostPID — can read /proc/1/mem of host
		"ESC_HOSTIPC_POD",       // hostIPC — shared memory with host
		"ESC_RUNTIME_PROBE",     // static risk + runtime signal
		"ESC_RUNTIME_PROC_ROOT": // /proc/1/root pivot, runtime confirmed
		return true
	default:
		return false
	}
}

func capabilityStateAmplifier(pod models.Pod, caps []models.PodCapability, hasObservedEgress bool) float64 {
	amp := 0.0
	for _, c := range caps {
		id := strings.ToUpper(strings.TrimSpace(c.CapabilityID))
		if id != "ESC_PRIV_POD" && id != "PRIVILEGED" && id != "CAP_SYS_ADMIN" && id != "NET_RAW" {
			continue
		}
		// NET_RAW is context-sensitive: only boost when network operation is plausibly usable.
		if id == "NET_RAW" && !pod.HostNetwork && !hasObservedEgress {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(c.State)) {
		case "chained", "exploited":
			amp += 0.10
		case "confirmed":
			amp += 0.05
		default:
			amp += 0.03
		}
	}
	return clampFloat(amp, 0, 0.25)
}

func graphNodeDisplayName(n GraphNode) string {
	if strings.TrimSpace(n.Label) != "" {
		return strings.TrimSpace(n.Label)
	}
	id := strings.TrimSpace(n.ID)
	if len(id) <= 16 {
		return id
	}
	return id[:14] + "…"
}

func buildAttackPathDescription(gp GraphPath, nodes []PathNode) string {
	classHuman := strings.ReplaceAll(strings.TrimSuffix(gp.Class, "_PATH"), "_", " ")
	if len(nodes) == 0 {
		return fmt.Sprintf("%s path", classHuman)
	}
	first, _ := nodes[0].Properties["name"].(string)
	last, _ := nodes[len(nodes)-1].Properties["name"].(string)
	netHint := ""
	for _, et := range gp.EdgeTypes {
		if et == EdgeTypeNetworkReachSoft {
			netHint = " (network=SOFT_ALLOW)"
			break
		}
		if et == EdgeTypeNetworkReach {
			netHint = " (network=ALLOW)"
			break
		}
	}
	return fmt.Sprintf("%s: %s → %s%s", classHuman, first, last, netHint)
}

func convertGraphPathsToAttackPaths(gps []GraphPath, g *AttackGraph) []AttackPath {
	out := make([]AttackPath, 0, len(gps))
	for _, gp := range gps {
		nodes := make([]PathNode, 0, len(gp.NodeIDs))
		edges := make([]PathEdge, 0, len(gp.EdgeTypes))
		capabilities := make([]string, 0)
		attackSteps := make([]string, 0)
		for _, nid := range gp.NodeIDs {
			n := g.Nodes[nid]
			if n.Type == NodeTypeCapability {
				capabilities = append(capabilities, n.ID)
			}
			if n.Type == NodeTypeAttackStep {
				attackSteps = append(attackSteps, n.ID)
			}
			display := graphNodeDisplayName(n)
			props := map[string]interface{}{
				"name":      display,
				"namespace": n.Namespace,
			}
			if n.Type == NodeTypePod {
				props["kind"] = "Pod"
			}
			// FIX E: bridge SemanticCaps from graph node to PathNode.Properties
			// so chain_detection.go can use rule-derived objectives/provides.
			if len(n.SemanticCaps) > 0 {
				props["semantic_caps"] = n.SemanticCaps
			}
			nodes = append(nodes, PathNode{
				ID:         n.ID,
				Type:       n.Type,
				Properties: props,
			})
		}
		weakestLink := 1.0
		for i, et := range gp.EdgeTypes {
			exploitability := edgeExploitability(g, gp.NodeIDs[i], gp.NodeIDs[i+1], et)
			weakestLink = math.Min(weakestLink, exploitability)
			edges = append(edges, PathEdge{Type: et, Source: gp.NodeIDs[i], Target: gp.NodeIDs[i+1]})
		}
		pathLen := len(gp.EdgeTypes)
		lengthPenalty := math.Exp(-0.25 * float64(pathLen))
		uncertainPenalty := 1.0
		if uncertainEdges := countUncertainEdges(gp.EdgeTypes); uncertainEdges > 0 {
			uncertainPenalty = math.Max(0.3, math.Exp(-0.35*float64(uncertainEdges)))
		}
		totalRisk := clampFloat(gp.Strength*10.0, 0, 10)
		impact := clampFloat(totalRisk/10.0, 0.2, 1.0)
		difficulty := clampFloat(1.0-gp.Strength, 0.1, 1.0)
		reachesNode := false
		for _, n := range nodes {
			if n.Type == NodeTypeNode {
				reachesNode = true
				break
			}
		}
		effectiveStrength := math.Min(gp.Strength, 0.9)
		injectScale := 0.5 + 0.5*effectiveStrength
		eInject := 0.25 * gp.Strength * injectScale
		rInject := 0.20 * gp.Strength * injectScale
		iInject := 0.0
		if reachesNode {
			iInject = 0.20 * injectScale
		}
		ePoints := eInject * 15.0
		rPoints := rInject * 15.0
		iPoints := iInject * 10.0
		networkDecision, reason, confidence := deriveFeasibility(gp.EdgeTypes)
		out = append(out, AttackPath{
			Nodes:       nodes,
			Edges:       edges,
			TotalRisk:   totalRisk,
			Difficulty:  difficulty,
			Impact:      impact,
			Length:      len(gp.EdgeTypes),
			Description: buildAttackPathDescription(gp, nodes),
			Explainability: &PathExplainability{
				Class:    gp.Class,
				Strength: math.Round(gp.Strength*1000) / 1000,
				Feasibility: PathFeasibility{
					NetworkDecision: networkDecision,
					Reason:          reason,
					Confidence:      math.Round(confidence*100) / 100,
				},
				Scoring: PathScoringBreakdown{
					WeakestLink:      math.Round(weakestLink*1000) / 1000,
					LengthPenalty:    math.Round(lengthPenalty*1000) / 1000,
					UncertainPenalty: math.Round(uncertainPenalty*1000) / 1000,
					FinalStrength:    math.Round(gp.Strength*1000) / 1000,
				},
				RiskInject: PathRiskInjection{
					E:                   math.Round(eInject*1000) / 1000,
					R:                   math.Round(rInject*1000) / 1000,
					I:                   math.Round(iInject*1000) / 1000,
					Scale:               math.Round(injectScale*1000) / 1000,
					EPoints:             math.Round(ePoints*100) / 100,
					RPoints:             math.Round(rPoints*100) / 100,
					IPoints:             math.Round(iPoints*100) / 100,
					EstimatedScoreDelta: math.Round((ePoints+rPoints+iPoints)*100) / 100,
				},
				Evidence: PathExplainabilityData{
					Capabilities: capabilities,
					AttackSteps:  attackSteps,
				},
			},
		})
	}
	return out
}

func edgeExploitability(g *AttackGraph, from, to, edgeType string) float64 {
	for _, e := range g.Adj[from] {
		if e.To == to && e.Type == edgeType {
			return clampFloat(e.Exploitability, 0.05, 1.0)
		}
	}
	return 0.5
}

func deriveFeasibility(edgeTypes []string) (string, string, float64) {
	for _, et := range edgeTypes {
		if et == EdgeTypeNetworkReachSoft {
			return "soft_allow", "new_pod_fallback", 0.55
		}
		if et == EdgeTypeNetworkReach {
			return "allow", "observed_egress", 0.85
		}
	}
	return "n/a", "not_required", 0.9
}

func exploitabilityByRisk(riskLevel string) float64 {
	switch strings.ToLower(riskLevel) {
	case "critical":
		return 0.95
	case "high":
		return 0.85
	case "medium":
		return 0.7
	default:
		return 0.5
	}
}

func (b *RelationalPathBuilder) enrichTopPodsFromInventory(ctx context.Context, pods []RiskyPod) {
	if b == nil || b.db == nil || len(pods) == 0 {
		return
	}
	for i := range pods {
		if strings.TrimSpace(pods[i].UID) == "" {
			continue
		}
		if strings.TrimSpace(pods[i].Name) != "" && strings.TrimSpace(pods[i].ServiceAccountName) != "" && strings.TrimSpace(pods[i].Namespace) != "" {
			continue
		}
		var p models.Pod
		if err := b.db.WithContext(ctx).Where("uid = ? AND deleted_at IS NULL", pods[i].UID).First(&p).Error; err != nil {
			continue
		}
		if strings.TrimSpace(pods[i].Name) == "" {
			pods[i].Name = p.Name
		}
		if strings.TrimSpace(pods[i].Namespace) == "" {
			pods[i].Namespace = p.Namespace
		}
		if strings.TrimSpace(pods[i].ServiceAccountName) == "" {
			pods[i].ServiceAccountName = p.ServiceAccount
		}
	}
}

func (b *RelationalPathBuilder) attachUnifiedV3Scores(ctx context.Context, pods []RiskyPod) {
	if b == nil || b.db == nil || len(pods) == 0 {
		return
	}
	if !b.db.Migrator().HasTable(&models.RiskScore{}) {
		return
	}
	for i := range pods {
		uid := strings.TrimSpace(pods[i].UID)
		if uid == "" {
			continue
		}
		var rs models.RiskScore
		if err := b.db.WithContext(ctx).
			Where("resource_uid = ? AND scorer_version = ? AND deleted_at IS NULL", uid, "v3").
			Order("calculated_at DESC").
			First(&rs).Error; err != nil {
			continue
		}
		sc := rs.TotalScore
		pods[i].UnifiedScore = &sc
	}
}

// GetSummary computes summary statistics for attack paths.
func (b *RelationalPathBuilder) GetSummary(ctx context.Context, clusterID string) (*AttackPathSummary, error) {
	paths, err := b.BuildAllPaths(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}
	return b.summarizePaths(ctx, paths)
}

func (b *RelationalPathBuilder) summarizePaths(ctx context.Context, paths []AttackPath) (*AttackPathSummary, error) {
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

	b.enrichTopPodsFromInventory(ctx, summary.TopPods)
	b.attachUnifiedV3Scores(ctx, summary.TopPods)

	return summary, nil
}

// GetChains returns detected chainable attack scenarios for a cluster.
func (b *RelationalPathBuilder) GetChains(ctx context.Context, clusterID string) ([]AttackChain, error) {
	chains, _, err := b.GetChainsWithPaths(ctx, clusterID)
	return chains, err
}

// GetChainsWithPaths returns detected chains and a stable path_id → AttackPath map (for resource interpretation / pivot counts).
func (b *RelationalPathBuilder) GetChainsWithPaths(ctx context.Context, clusterID string) ([]AttackChain, map[string]AttackPath, error) {
	snap, err := b.loadClusterPathSnapshot(ctx, clusterID)
	if err != nil {
		return nil, nil, err
	}
	paths, err := b.BuildAllPaths(ctx, clusterID, false)
	if err != nil {
		return nil, nil, err
	}
	pathByID := make(map[string]AttackPath, len(paths))
	for i := range paths {
		pid := fmt.Sprintf("p%d", i)
		paths[i].PathID = pid
		pathByID[pid] = paths[i]
	}
	chains := DetectChainsWithHints(paths, snap.HardeningHints)
	EnrichAttackChainsWithRuntimeMitre(b.db, clusterID, chains, paths)
	return chains, pathByID, nil
}

// GetObjectives groups chain scenarios into actionable attack objectives.
func (b *RelationalPathBuilder) GetObjectives(ctx context.Context, clusterID string) ([]AttackObjective, error) {
	chains, err := b.GetChains(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	return aggregateAttackPathObjectives(chains), nil
}

func aggregateAttackPathObjectives(chains []AttackChain) []AttackObjective {
	if len(chains) == 0 {
		return []AttackObjective{}
	}
	type agg struct {
		chainCount   int
		pathCount    int
		maxStrength  float64
		pods         map[string]bool
		confidenceHi int
		confidenceMd int
		confidenceLo int
	}
	byObjective := map[string]*agg{}
	for _, ch := range chains {
		objective := strings.TrimSpace(ch.Objective + "|" + ch.FinalTarget)
		cur, ok := byObjective[objective]
		if !ok {
			cur = &agg{pods: map[string]bool{}}
			byObjective[objective] = cur
		}
		cur.chainCount++
		cur.pathCount += len(ch.Paths)
		for _, pid := range ch.Paths {
			cur.pods[pid] = true
		}
		strength := clampFloat(ch.ChainStrength, 0, 1)
		switch confidenceBandString(ch.Confidence) {
		case "high":
			cur.confidenceHi++
		case "medium":
			cur.confidenceMd++
		default:
			cur.confidenceLo++
		}
		if strength > cur.maxStrength {
			cur.maxStrength = strength
		}
	}
	out := make([]AttackObjective, 0, len(byObjective))
	for key, a := range byObjective {
		parts := strings.SplitN(key, "|", 2)
		name := parts[0]
		finalTarget := ""
		if len(parts) > 1 {
			finalTarget = parts[1]
		}
		pods := make([]string, 0, len(a.pods))
		for uid := range a.pods {
			pods = append(pods, uid)
		}
		sort.Strings(pods)
		priority := "low"
		switch {
		case a.maxStrength >= 0.35:
			priority = "high"
		case a.maxStrength >= 0.2:
			priority = "medium"
		}
		confidence := "low"
		if a.confidenceHi >= a.confidenceMd && a.confidenceHi >= a.confidenceLo {
			confidence = "high"
		} else if a.confidenceMd >= a.confidenceLo {
			confidence = "medium"
		}
		// C13: amplify confidence when >=3 chains converge on same final target.
		if a.chainCount >= 3 {
			confidence = bumpConfidence(confidence)
		}
		out = append(out, AttackObjective{
			Objective:    name,
			FinalTarget:  finalTarget,
			Priority:     priority,
			ChainCount:   a.chainCount,
			PathCount:    a.pathCount,
			MaxStrength:  math.Round(a.maxStrength*1000) / 1000,
			InvolvedPods: len(a.pods),
			PodUIDs:      pods,
			Confidence:   confidence,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		rank := func(p string) int {
			switch p {
			case "high":
				return 0
			case "medium":
				return 1
			default:
				return 2
			}
		}
		ri, rj := rank(out[i].Priority), rank(out[j].Priority)
		if ri != rj {
			return ri < rj
		}
		return out[i].MaxStrength > out[j].MaxStrength
	})
	return out
}

func confidenceBand(v float64) string {
	switch {
	case v >= 0.8:
		return "high"
	case v >= 0.6:
		return "medium"
	default:
		return "low"
	}
}

func confidenceBandString(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

func bumpConfidence(v string) string {
	switch v {
	case "low":
		return "medium"
	case "medium":
		return "high"
	default:
		return "high"
	}
}

// BuildGraphData returns nodes and links for D3 visualization from all computed paths.
func (b *RelationalPathBuilder) BuildGraphData(ctx context.Context, clusterID string) (map[string]interface{}, error) {
	paths, err := b.BuildAllPaths(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}
	return buildGraphDataFromPaths(paths), nil
}

func buildGraphDataFromPaths(paths []AttackPath) map[string]interface{} {
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
	}
}

// BuildAttackPathsViewBundle runs a single BuildAllPaths pass and derives graph, summary, chains, and objectives.
func (b *RelationalPathBuilder) BuildAttackPathsViewBundle(ctx context.Context, clusterID string) (*AttackPathsViewBundle, error) {
	// Load snapshot first so we can pass HardeningHints to chain detection (FIX I).
	snap, err := b.loadClusterPathSnapshot(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if err := cleanupStaleAttackPaths(ctx, b.db, clusterID); err != nil {
		log.Printf("[RelationalPathBuilder] stale attack_paths cleanup failed: %v", err)
	}
	paths, err := b.BuildAllPaths(ctx, clusterID, false)
	if err != nil {
		return nil, err
	}
	for i := range paths {
		paths[i].PathID = fmt.Sprintf("p%d", i)
	}
	summary, err := b.summarizePaths(ctx, paths)
	if err != nil {
		return nil, err
	}
	// FIX I: use context-aware chain detection with cluster hardening hints.
	chains := DetectChainsWithHints(paths, snap.HardeningHints)
	EnrichAttackChainsWithRuntimeMitre(b.db, clusterID, chains, paths)
	return &AttackPathsViewBundle{
		Graph:      buildGraphDataFromPaths(paths),
		Summary:    summary,
		Chains:     chains,
		Objectives: aggregateAttackPathObjectives(chains),
		Paths:      paths,
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

// persistAttackPaths upserts computed attack paths into the attack_paths table.
// If the table does not yet exist (pre-migration environment), this is a no-op.
func persistAttackPaths(ctx context.Context, db *gorm.DB, podUID string, paths []AttackPath) error {
	if !db.Migrator().HasTable("attack_paths") {
		return nil
	}

	newByPathID := map[string]AttackPath{}
	for i, p := range paths {
		nodesJSON, _ := json.Marshal(p.Nodes)
		edgesJSON, _ := json.Marshal(p.Edges)

		// Stable path ID: based on position so re-computation produces the same key.
		pathID := fmt.Sprintf("%s-path-%d", podUID, i)
		newByPathID[pathID] = AttackPath{
			Nodes:       p.Nodes,
			Edges:       p.Edges,
			TotalRisk:   p.TotalRisk,
			Difficulty:  p.Difficulty,
			Impact:      p.Impact,
			Length:      p.Length,
			Description: p.Description,
		}

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

	// Hysteresis for disappeared paths: keep for grace period with decay to avoid UI/score flapping.
	grace := attackPathHysteresisGraceFromEnv()
	decay := attackPathHysteresisDecayFromEnv()
	if grace > 0 {
		var existing []models.AttackPath
		if err := db.WithContext(ctx).Where("pod_uid = ?", podUID).Find(&existing).Error; err == nil {
			now := time.Now()
			for _, old := range existing {
				if _, stillPresent := newByPathID[old.PathID]; stillPresent {
					continue
				}
				age := now.Sub(old.UpdatedAt)
				if age > grace {
					_ = db.WithContext(ctx).Where("id = ?", old.ID).Delete(&models.AttackPath{}).Error
					continue
				}
				old.TotalRisk = clampFloat(old.TotalRisk*decay, 0, 10)
				old.Impact = clampFloat(old.Impact*decay, 0, 1)
				old.Difficulty = clampFloat(1.0-(1.0-old.Difficulty)*decay, 0.1, 1.0)
				old.Description = strings.TrimSpace(old.Description + " [hysteresis]")
				_ = db.WithContext(ctx).Save(&old).Error
			}
		}
	}
	return nil
}

func cleanupStaleAttackPaths(ctx context.Context, db *gorm.DB, clusterID string) error {
	if db == nil || !db.Migrator().HasTable("attack_paths") || !db.Migrator().HasTable("pods") {
		return nil
	}
	stalePredicate := `NOT EXISTS (
		SELECT 1 FROM pods
		WHERE pods.uid = attack_paths.pod_uid
			AND pods.deleted_at IS NULL
	)`
	q := db.WithContext(ctx).Where(stalePredicate)
	if strings.TrimSpace(clusterID) != "" {
		q = q.Where(`pod_uid IN (
			SELECT uid FROM pods
			WHERE cluster_id = ?
		)`, clusterID)
	}
	if err := q.Delete(&models.AttackPath{}).Error; err != nil {
		return fmt.Errorf("delete stale attack paths: %w", err)
	}
	return nil
}

func attackPathHysteresisGraceFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_ATTACK_PATH_HYSTERESIS_GRACE"))
	if raw == "" {
		return 60 * time.Minute
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 60 * time.Minute
	}
	return d
}

func attackPathHysteresisDecayFromEnv() float64 {
	raw := strings.TrimSpace(os.Getenv("FORTUNA_ATTACK_PATH_HYSTERESIS_DECAY"))
	if raw == "" {
		return 0.7
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 || v > 1 {
		return 0.7
	}
	return v
}

func (b *RelationalPathBuilder) loadDenyCache(ctx context.Context) map[string]time.Time {
	out := map[string]time.Time{}
	if b == nil || b.db == nil || !b.db.Migrator().HasTable(&models.RuntimeSignal{}) {
		return out
	}
	type denyRow struct {
		PodUID     string
		Evidence   string
		CreatedAt  time.Time
		SignalType string
	}
	var rows []denyRow
	since := time.Now().Add(-attackPathDenyTTLFromEnv())
	_ = b.db.WithContext(ctx).
		Model(&models.RuntimeSignal{}).
		Select("pod_uid, evidence, created_at, signal_type").
		Where("created_at >= ? AND signal_type IN ?", since, []string{"NETWORK_POLICY_DENY", "NETWORK_DENY"}).
		Find(&rows).Error
	for _, r := range rows {
		targetUID := ""
		if strings.TrimSpace(r.Evidence) != "" {
			var ev map[string]interface{}
			if err := json.Unmarshal([]byte(r.Evidence), &ev); err == nil {
				if v, ok := ev["targetPodUID"].(string); ok {
					targetUID = strings.TrimSpace(v)
				}
			}
		}
		if targetUID == "" {
			continue
		}
		key := r.PodUID + "->" + targetUID
		if prev, ok := out[key]; !ok || r.CreatedAt.After(prev) {
			out[key] = r.CreatedAt
		}
	}
	return out
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
