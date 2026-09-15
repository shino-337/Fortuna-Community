package graph

import "testing"

func TestAttackPathCacheIsolation(t *testing.T) {
	t.Setenv("FORTUNA_ATTACK_PATH_BUILD_CACHE_TTL", "1m")
	clearAttackPathBuildCache()
	t.Cleanup(clearAttackPathBuildCache)
	source := []AttackPath{{Nodes: []PathNode{{ID: "private", Properties: map[string]interface{}{"label": "original"}}}}}
	setCachedAttackPathsAll("", source)
	if _, ok := getCachedAttackPathsAll("__all_clusters__"); ok {
		t.Fatal("global cache collides with literal cluster ID")
	}
	source[0].Nodes[0].ID = "mutated-source"
	first, ok := getCachedAttackPathsAll("")
	if !ok || first[0].Nodes[0].ID != "private" {
		t.Fatal("cache shares input nodes")
	}
	first[0].Nodes[0].Properties["label"] = "redacted"
	second, ok := getCachedAttackPathsAll("")
	if !ok || second[0].Nodes[0].Properties["label"] != "original" {
		t.Fatal("request mutation modified cache")
	}
}
