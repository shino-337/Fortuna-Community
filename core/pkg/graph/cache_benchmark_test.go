package graph

import (
	"fmt"
	"testing"
)

func benchmarkPaths() []AttackPath {
	paths := make([]AttackPath, 500)
	for i := range paths {
		paths[i] = AttackPath{PathID: fmt.Sprint(i), Nodes: []PathNode{{ID: "pod", Type: "Pod", Properties: map[string]any{"cluster_id": "benchmark", "name": "pod", "nested": map[string]any{"labels": []any{"a", "b"}}}}, {ID: "sa", Type: "ServiceAccount", Properties: map[string]any{"cluster_id": "benchmark"}}}, Edges: []PathEdge{{Source: "pod", Target: "sa", Type: "SERVICE_ACCOUNT_ACCESS", Properties: map[string]any{"reason": "RBAC evidence"}}}, Length: 1}
	}
	return paths
}
func BenchmarkAttackPathCacheRead(b *testing.B) {
	b.Setenv("FORTUNA_ATTACK_PATH_BUILD_CACHE_TTL", "1h")
	clearAttackPathBuildCache()
	setCachedAttackPathsAll("benchmark", benchmarkPaths())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		paths, ok := getCachedAttackPathsAll("benchmark")
		if !ok || len(paths) != 500 {
			b.Fatal("cache miss")
		}
	}
}
