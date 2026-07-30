package authorization

import "testing"

func TestOperationalScopeFromDocument(t *testing.T) {
	s := OperationalScopeFromDocument(`{"clusters":["c1"],"namespaces":["ns-a"],"environments":["prod"],"tenants":["team-1"]}`)
	if !s.Restricted {
		t.Fatal("expected restricted")
	}
	if len(s.Clusters) != 1 || s.Clusters[0] != "c1" {
		t.Fatalf("clusters: %+v", s.Clusters)
	}
	if len(s.Namespaces) != 1 || s.Namespaces[0] != "ns-a" {
		t.Fatalf("namespaces: %+v", s.Namespaces)
	}
	if len(s.Teams) != 1 || s.Teams[0] != "team-1" {
		t.Fatalf("teams: %+v", s.Teams)
	}
	if len(s.Environments) != 1 || s.Environments[0] != "prod" {
		t.Fatalf("environments: %+v", s.Environments)
	}
}

func TestOperationalScopeLegacyClusterIDs(t *testing.T) {
	s := OperationalScopeFromDocument(`{"cluster_ids":["1","2"]}`)
	if len(s.Clusters) != 2 {
		t.Fatalf("clusters: %+v", s.Clusters)
	}
}

func TestOperationalScopeEmptyUnrestricted(t *testing.T) {
	s := OperationalScopeFromDocument("{}")
	if s.Restricted {
		t.Fatal("empty scope should not be restricted")
	}
}
