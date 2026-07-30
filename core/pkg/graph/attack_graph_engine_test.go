package graph

import "testing"

func TestBuildTopPathsAdaptive_BasicTraversal(t *testing.T) {
	g := NewAttackGraph()
	g.AddNode(GraphNode{ID: "pod-1", Type: NodeTypePod, Namespace: "ns-a"})
	g.AddNode(GraphNode{ID: "sa-1", Type: NodeTypeServiceAccount, Namespace: "ns-a"})
	g.AddNode(GraphNode{ID: "cr-1", Type: NodeTypeClusterRole, Namespace: ""})

	g.AddEdge(GraphEdge{From: "pod-1", To: "sa-1", Type: EdgeTypeServiceAccount, Exploitability: 0.9})
	g.AddEdge(GraphEdge{From: "sa-1", To: "cr-1", Type: EdgeTypeGrantsRole, Exploitability: 0.8})

	paths := g.BuildTopPathsAdaptive(
		"pod-1",
		map[string]bool{NodeTypeClusterRole: true},
		true,
		PathBuildOptions{},
		nil,
	)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if paths[0].Strength <= 0.40 || paths[0].Strength >= 0.43 {
		t.Fatalf("unexpected path strength: %.3f", paths[0].Strength)
	}
}

func TestBuildTopPathsAdaptive_WeakestLinkPenalty(t *testing.T) {
	g := NewAttackGraph()
	g.AddNode(GraphNode{ID: "pod", Type: NodeTypePod, Namespace: "ns-a"})
	g.AddNode(GraphNode{ID: "sa", Type: NodeTypeServiceAccount, Namespace: "ns-a"})
	g.AddNode(GraphNode{ID: "role-weak", Type: NodeTypeRole, Namespace: "ns-a"})
	g.AddNode(GraphNode{ID: "role-strong", Type: NodeTypeRole, Namespace: "ns-a"})

	g.AddEdge(GraphEdge{From: "pod", To: "sa", Type: EdgeTypeServiceAccount, Exploitability: 0.9})
	g.AddEdge(GraphEdge{From: "sa", To: "role-weak", Type: EdgeTypeGrantsRole, Exploitability: 0.2})
	g.AddEdge(GraphEdge{From: "sa", To: "role-strong", Type: EdgeTypeGrantsRole, Exploitability: 0.9})

	paths := g.BuildTopPathsAdaptive(
		"pod",
		map[string]bool{NodeTypeRole: true},
		false,
		PathBuildOptions{},
		nil,
	)
	if len(paths) < 2 {
		t.Fatalf("expected at least 2 paths, got %d", len(paths))
	}
	if paths[0].Strength <= paths[1].Strength {
		t.Fatalf("expected strong path > weak path: %.3f <= %.3f", paths[0].Strength, paths[1].Strength)
	}
}

func TestBuildTopPathsAdaptive_FeasibilityFilter(t *testing.T) {
	g := NewAttackGraph()
	g.AddNode(GraphNode{ID: "pod-a", Type: NodeTypePod, Namespace: "ns-a"})
	g.AddNode(GraphNode{ID: "sa-a", Type: NodeTypeServiceAccount, Namespace: "ns-a"})
	g.AddNode(GraphNode{ID: "role-b", Type: NodeTypeRole, Namespace: "ns-b"})

	g.AddEdge(GraphEdge{From: "pod-a", To: "sa-a", Type: EdgeTypeServiceAccount, Exploitability: 1.0})
	g.AddEdge(GraphEdge{From: "sa-a", To: "role-b", Type: EdgeTypeGrantsRole, Exploitability: 1.0})

	paths := g.BuildTopPathsAdaptive(
		"pod-a",
		map[string]bool{NodeTypeRole: true},
		false,
		PathBuildOptions{},
		func(edge GraphEdge, from, to GraphNode) bool {
			// Example feasibility rule: disallow lateral movement into a different namespace.
			if from.Namespace != "" && to.Namespace != "" && from.Namespace != to.Namespace {
				return false
			}
			return true
		},
	)
	if len(paths) != 0 {
		t.Fatalf("expected no feasible path, got %d", len(paths))
	}
}
