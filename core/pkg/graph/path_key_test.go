package graph

import (
	"strings"
	"testing"
)

func keyTestPath(entry string) AttackPath {
	return AttackPath{
		Nodes: []PathNode{
			{ID: entry, Type: "Pod"},
			{ID: "sa-web", Type: "ServiceAccount"},
			{ID: "crb-ci-admin", Type: "ClusterRoleBinding"},
			{ID: "cr-cluster-admin", Type: "ClusterRole"},
		},
		Edges: []PathEdge{
			{Type: "USES_SERVICE_ACCOUNT", Source: entry, Target: "sa-web"},
			{Type: "RBAC_BINDING", Source: "sa-web", Target: "crb-ci-admin"},
			{Type: "GRANTS_ROLE", Source: "crb-ci-admin", Target: "cr-cluster-admin"},
		},
		TotalRisk: 9.4,
	}
}

func TestStablePathIDIgnoresPositionAndEdgeOrder(t *testing.T) {
	a := keyTestPath("pod-frontend")
	b := keyTestPath("pod-frontend")
	b.Edges[0], b.Edges[2] = b.Edges[2], b.Edges[0]
	b.TotalRisk = 7.1 // risk is not part of identity

	if got, want := StablePathID(b), StablePathID(a); got != want {
		t.Fatalf("same path, different edge order: %s != %s", got, want)
	}
	if !strings.HasPrefix(StablePathID(a), "ap_") {
		t.Fatalf("unexpected id format %q", StablePathID(a))
	}

	// The id must not depend on where the path sits in the list.
	first := StablePathIDs([]AttackPath{a, keyTestPath("pod-runner")})
	second := StablePathIDs([]AttackPath{keyTestPath("pod-runner"), a})
	if first[0] != second[1] || first[1] != second[0] {
		t.Fatalf("ids changed with list order: %v vs %v", first, second)
	}
}

func TestStablePathIDDiffersByEntryAndHops(t *testing.T) {
	base := StablePathID(keyTestPath("pod-frontend"))
	if StablePathID(keyTestPath("pod-runner")) == base {
		t.Fatal("different entry pods must not share an id")
	}
	longer := keyTestPath("pod-frontend")
	longer.Nodes = append(longer.Nodes, PathNode{ID: "secret-db", Type: "Secret"})
	longer.Edges = append(longer.Edges, PathEdge{Type: "CAN_READ", Source: "cr-cluster-admin", Target: "secret-db"})
	if StablePathID(longer) == base {
		t.Fatal("an extra hop must change the id")
	}
}

func TestStablePathIDsSuffixesExactDuplicates(t *testing.T) {
	ids := StablePathIDs([]AttackPath{keyTestPath("pod-a"), keyTestPath("pod-a"), keyTestPath("pod-a")})
	if ids[1] != ids[0]+"-2" || ids[2] != ids[0]+"-3" {
		t.Fatalf("duplicates not suffixed: %v", ids)
	}
}

func TestChainsReferenceStablePathIDs(t *testing.T) {
	paths := []AttackPath{keyTestPath("pod-a"), keyTestPath("pod-b")}
	AssignStablePathIDs(paths)
	norm := normalizePaths(paths)
	if len(norm) != 2 {
		t.Fatalf("expected 2 normalized paths, got %d", len(norm))
	}
	for i := range norm {
		if norm[i].PathID != paths[i].PathID {
			t.Fatalf("chain detection uses %q, path carries %q", norm[i].PathID, paths[i].PathID)
		}
	}
}
