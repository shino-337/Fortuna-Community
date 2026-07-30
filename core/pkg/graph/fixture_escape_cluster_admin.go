package graph

// EscapeClusterAdminFixtureGraph builds a minimal in-memory graph used by golden tests:
// pod→SA→RBAC→cluster-admin plus pod→hostPath capability→node (escape).
// Pod and capability share namespace so cap→node edges pass production feasibility checks.
func EscapeClusterAdminFixtureGraph() *AttackGraph {
	g := NewAttackGraph()
	const (
		podID  = "fixture-pod-1"
		saID   = "fixture-sa-1"
		capID  = "cap:fixture-pod-1:ESC_HOSTPATH_NODE"
		nodeID = "node:worker-1"
		crbID  = "fixture-crb-admin"
		crID   = "role:cluster-admin"
	)
	g.AddNode(GraphNode{ID: podID, Type: NodeTypePod, Namespace: "ns-fix", Label: "workload"})
	g.AddNode(GraphNode{ID: saID, Type: NodeTypeServiceAccount, Namespace: "ns-fix", Label: "default"})
	g.AddEdge(GraphEdge{From: podID, To: saID, Type: EdgeTypeServiceAccount, Exploitability: 0.92})

	g.AddNode(GraphNode{ID: capID, Type: NodeTypeCapability, Namespace: "ns-fix", Label: "ESC_HOSTPATH_NODE"})
	g.AddEdge(GraphEdge{From: podID, To: capID, Type: EdgeTypeHostAccess, Exploitability: 0.88})
	g.AddNode(GraphNode{ID: nodeID, Type: NodeTypeNode, Namespace: "ns-fix", Label: "worker-1"})
	g.AddEdge(GraphEdge{From: capID, To: nodeID, Type: EdgeTypeLateralMove, Exploitability: 0.88})

	g.AddNode(GraphNode{ID: crbID, Type: NodeTypeClusterBinding, Namespace: "", Label: "system:controller:fixture"})
	g.AddNode(GraphNode{ID: crID, Type: NodeTypeClusterRole, Namespace: "", Label: "cluster-admin",
		SemanticCaps: []string{"IDENTITY_FORGE"},
	})
	g.AddEdge(GraphEdge{From: saID, To: crbID, Type: EdgeTypeRbacBinding, Exploitability: 0.91})
	g.AddEdge(GraphEdge{From: crbID, To: crID, Type: EdgeTypeGrantsRole, Exploitability: 0.9})
	return g
}

// AttackPathsFromEscapeClusterAdminFixture runs the same path pipeline as bundle code:
// BuildTopPathsAdaptive → convertGraphPathsToAttackPaths.
func AttackPathsFromEscapeClusterAdminFixture(startPod string) []AttackPath {
	g := EscapeClusterAdminFixtureGraph()
	targetTypes := map[string]bool{
		NodeTypeRole:        true,
		NodeTypeClusterRole: true,
		NodeTypeNode:        true,
	}
	opts := PathBuildOptions{
		MaxTotalPaths:   10,
		MaxPathsPerType: 3,
		BeamWidth:       8,
		MinStrength:     0.12,
	}
	gps := g.BuildTopPathsAdaptive(
		startPod,
		targetTypes,
		true,
		opts,
		func(edge GraphEdge, from, to GraphNode) bool {
			if from.Namespace != "" && to.Namespace != "" && from.Namespace != to.Namespace &&
				edge.Type != EdgeTypeRbacBinding && edge.Type != EdgeTypeGrantsRole {
				return false
			}
			return true
		},
	)
	return convertGraphPathsToAttackPaths(gps, g)
}
