package authorization

import "testing"

func TestScopeDocumentValidate(t *testing.T) {
	d := ScopeDocument{Labels: map[string]string{"a": "b"}}
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	big := ScopeDocument{Namespaces: make([]string, 600)}
	if err := big.Validate(); err == nil {
		t.Fatal("expected error")
	}
}


func TestParseScopeDocumentStrictRejectsMalformedAuthorizationShape(t *testing.T) {
	tests := []string{
		`{"clusters":"cluster-a"}`,
		`{"cluster_ids":{"id":"cluster-a"}}`,
		`{"labels":["not-a-map"]}`,
		`{"clusters":null}`,
		`{"clustres":["cluster-a"]}`,
		`null`,
	}
	for _, raw := range tests {
		if _, err := ParseScopeDocumentStrict(raw); err == nil {
			t.Fatalf("expected malformed scope to be rejected: %s", raw)
		}
		doc := ParseScopeDocument(raw)
		if !doc.RestrictsClusters() || doc.ClusterAllowed("cluster-a") {
			t.Fatalf("malformed persisted scope must fail closed: %s -> %+v", raw, doc)
		}
	}
}

func TestParseScopeDocumentStrictAcceptsCurrentAndLegacyClusterLists(t *testing.T) {
	for _, raw := range []string{
		`{"clusters":["a","b"]}`,
		`{"cluster_ids":["a","b"]}`,
		`{}`,
	} {
		doc, err := ParseScopeDocumentStrict(raw)
		if err != nil {
			t.Fatalf("valid scope rejected %s: %v", raw, err)
		}
		if err := doc.Validate(); err != nil {
			t.Fatalf("valid scope failed validation %s: %v", raw, err)
		}
	}
}


func TestPersistedUnenforcedScopeFailsClosedForClusterAuthorization(t *testing.T) {
	for _, raw := range []string{
		`{"namespaces":["prod"]}`,
		`{"environments":["prod"]}`,
		`{"labels":{"tier":"critical"}}`,
		`{"clusters":["cluster-a"],"namespaces":["prod"]}`,
	} {
		doc := ParseScopeDocument(raw)
		if !doc.RestrictsClusters() {
			t.Fatalf("unenforced persisted scope must be restrictive: %s", raw)
		}
		if doc.ClusterAllowed("cluster-a") {
			t.Fatalf("unenforced persisted scope must deny cluster access: %s", raw)
		}
		ids := doc.ClusterIDs()
		if len(ids) != 1 || ids[0] != "__invalid_scope__" {
			t.Fatalf("unenforced persisted scope must expose deny-all effective cluster ids: %s -> %#v", raw, ids)
		}
	}
}

func TestEmptyClusterAllowListIsRejectedAndFailsClosed(t *testing.T) {
	for _, raw := range []string{`{"clusters":[]}`, `{"cluster_ids":[]}`, `{"clusters":[],"namespaces":[]}`} {
		if _, err := ParseScopeDocumentStrict(raw); err == nil {
			t.Fatalf("empty cluster allow-list must be rejected on write: %s", raw)
		}
		doc := ParseScopeDocument(raw)
		if !doc.RestrictsClusters() || doc.ClusterAllowed("cluster-a") {
			t.Fatalf("persisted empty cluster allow-list must deny every cluster: %s -> %+v", raw, doc)
		}
	}
	// Omitting the field still means unrestricted.
	if doc := ParseScopeDocument(`{"namespaces":[]}`); doc.RestrictsClusters() {
		t.Fatal("scope without a cluster field must stay unrestricted")
	}
}
