package graph

// RbacOnlyClusterAdminFixtureGraph is pod→SA→CRB→cluster-admin only (no escape / hostPath / node).
// Used to assert we do not imply escape semantics when the graph has no escape edges.
func RbacOnlyClusterAdminFixtureGraph() *AttackGraph {
	g := NewAttackGraph()
	const (
		podID = "fixture-rbac-pod-1"
		saID  = "fixture-rbac-sa-1"
		crbID = "fixture-rbac-crb"
		crID  = "role:cluster-admin"
	)
	g.AddNode(GraphNode{ID: podID, Type: NodeTypePod, Namespace: "ns-rbac", Label: "workload"})
	g.AddNode(GraphNode{ID: saID, Type: NodeTypeServiceAccount, Namespace: "ns-rbac", Label: "default"})
	g.AddEdge(GraphEdge{From: podID, To: saID, Type: EdgeTypeServiceAccount, Exploitability: 0.92})

	g.AddNode(GraphNode{ID: crbID, Type: NodeTypeClusterBinding, Namespace: "", Label: "rbac:fixture"})
	g.AddNode(GraphNode{ID: crID, Type: NodeTypeClusterRole, Namespace: "", Label: "cluster-admin",
		SemanticCaps: []string{"IDENTITY_FORGE"},
	})
	g.AddEdge(GraphEdge{From: saID, To: crbID, Type: EdgeTypeRbacBinding, Exploitability: 0.91})
	g.AddEdge(GraphEdge{From: crbID, To: crID, Type: EdgeTypeGrantsRole, Exploitability: 0.9})
	return g
}

// AttackPathsFromRbacOnlyClusterAdminFixture runs BuildTopPathsAdaptive for the RBAC-only graph.
func AttackPathsFromRbacOnlyClusterAdminFixture(startPod string) []AttackPath {
	g := RbacOnlyClusterAdminFixtureGraph()
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
