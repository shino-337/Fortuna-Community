package api

import (
	"fmt"
	"sort"

	"github.com/fortuna/core/pkg/graph"
)

// Graph responses are derived from every attack path in a cluster, so their
// size grows with inventory. These helpers cap them at the API boundary while
// keeping the payload self-consistent (no dangling links or chain references).

func graphRiskRank(v interface{}) int {
	switch fmt.Sprint(v) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func graphLinkValue(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

// capGraphData bounds a {"nodes": [...], "links": [...]} payload. When either
// list exceeds its cap, the highest-risk nodes are kept, links are restricted
// to kept endpoints (highest value first), and "truncated", "totalNodes" and
// "totalLinks" are added. Payloads within the caps are returned unchanged.
func capGraphData(data map[string]interface{}, maxNodes, maxLinks int) map[string]interface{} {
	if data == nil {
		return data
	}
	nodes, _ := data["nodes"].([]map[string]interface{})
	links, _ := data["links"].([]map[string]interface{})
	if len(nodes) <= maxNodes && len(links) <= maxLinks {
		return data
	}
	totalNodes, totalLinks := len(nodes), len(links)

	keptNodes := nodes
	if len(nodes) > maxNodes {
		sorted := append([]map[string]interface{}(nil), nodes...)
		sort.SliceStable(sorted, func(i, j int) bool {
			ri, rj := graphRiskRank(sorted[i]["risk"]), graphRiskRank(sorted[j]["risk"])
			if ri != rj {
				return ri < rj
			}
			return fmt.Sprint(sorted[i]["id"]) < fmt.Sprint(sorted[j]["id"])
		})
		keptNodes = sorted[:maxNodes]
	}
	keep := make(map[string]struct{}, len(keptNodes))
	for _, n := range keptNodes {
		keep[fmt.Sprint(n["id"])] = struct{}{}
	}

	keptLinks := make([]map[string]interface{}, 0, len(links))
	for _, l := range links {
		_, okS := keep[fmt.Sprint(l["source"])]
		_, okT := keep[fmt.Sprint(l["target"])]
		if okS && okT {
			keptLinks = append(keptLinks, l)
		}
	}
	if len(keptLinks) > maxLinks {
		sort.SliceStable(keptLinks, func(i, j int) bool {
			vi, vj := graphLinkValue(keptLinks[i]["value"]), graphLinkValue(keptLinks[j]["value"])
			if vi != vj {
				return vi > vj
			}
			ki := fmt.Sprint(keptLinks[i]["source"], "->", keptLinks[i]["target"], ":", keptLinks[i]["type"])
			kj := fmt.Sprint(keptLinks[j]["source"], "->", keptLinks[j]["target"], ":", keptLinks[j]["type"])
			return ki < kj
		})
		keptLinks = keptLinks[:maxLinks]
	}

	out := make(map[string]interface{}, len(data)+3)
	for k, v := range data {
		out[k] = v
	}
	out["nodes"] = keptNodes
	out["links"] = keptLinks
	out["truncated"] = true
	out["totalNodes"] = totalNodes
	out["totalLinks"] = totalLinks
	return out
}

// capAttackPaths keeps at most max paths, always keeping paths whose PathID is
// in mustKeep, then the highest TotalRisk ones. The original order (and so the
// stable p0..pN PathIDs) is preserved.
func capAttackPaths(paths []graph.AttackPath, max int, mustKeep map[string]struct{}) ([]graph.AttackPath, bool) {
	if len(paths) <= max {
		return paths, false
	}
	idx := make([]int, len(paths))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		_, ka := mustKeep[paths[idx[a]].PathID]
		_, kb := mustKeep[paths[idx[b]].PathID]
		if ka != kb {
			return ka
		}
		return paths[idx[a]].TotalRisk > paths[idx[b]].TotalRisk
	})
	selected := idx[:max]
	sort.Ints(selected)
	out := make([]graph.AttackPath, 0, max)
	for _, i := range selected {
		out = append(out, paths[i])
	}
	return out, true
}

// capAttackPathsBundle bounds the bundle's chains, paths and graph. Paths
// referenced by kept chains are always retained.
func capAttackPathsBundle(b *graph.AttackPathsViewBundle) bool {
	if b == nil {
		return false
	}
	truncated := false
	if len(b.Chains) > attackChainsMax {
		b.Chains = b.Chains[:attackChainsMax]
		truncated = true
	}
	referenced := make(map[string]struct{})
	for _, ch := range b.Chains {
		for _, id := range ch.Paths {
			referenced[id] = struct{}{}
		}
	}
	var cut bool
	b.Paths, cut = capAttackPaths(b.Paths, attackBundleMaxPaths, referenced)
	truncated = truncated || cut
	b.Graph = capGraphData(b.Graph, graphMaxNodes, graphMaxLinks)
	if t, ok := b.Graph["truncated"].(bool); ok && t {
		truncated = true
	}
	return truncated
}
