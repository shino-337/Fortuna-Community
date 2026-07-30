package graph

import (
	"math"
	"sort"
)

const (
	PathClassEscape    = "ESCAPE_PATH"
	PathClassLateral   = "LATERAL_PATH"
	PathClassDataExfil = "DATA_EXFIL_PATH"
	PathClassPrivEsc   = "PRIV_ESC_PATH"
)

// GraphNode is a compact in-memory reasoning node.
type GraphNode struct {
	ID        string
	Type      string
	Namespace string
	// Label is operator-facing display text (pod name, SA name, role name, …). When empty, consumers fall back to ID.
	Label string
	// SemanticCaps carries semantic capability types derived from RBAC rules
	// via rbac.DeriveCapabilities. Used by deriveProvides to emit accurate
	// provides for role nodes without relying on role name string matching.
	// Intentionally excluded from JSON (engine-internal only).
	SemanticCaps []string
}

// GraphEdge is a typed directed edge with feasibility semantics.
type GraphEdge struct {
	From           string
	To             string
	Type           string
	Exploitability float64
	Preconditions  []string
}

// GraphPath is a deterministic traversal result.
type GraphPath struct {
	NodeIDs   []string
	EdgeTypes []string
	Strength  float64
	Class     string
}

// AttackGraph is an in-memory typed graph for attack-path generation.
type AttackGraph struct {
	Nodes map[string]GraphNode
	Adj   map[string][]GraphEdge
}

func NewAttackGraph() *AttackGraph {
	return &AttackGraph{
		Nodes: map[string]GraphNode{},
		Adj:   map[string][]GraphEdge{},
	}
}

func (g *AttackGraph) AddNode(n GraphNode) {
	if n.ID == "" {
		return
	}
	g.Nodes[n.ID] = n
}

func (g *AttackGraph) AddEdge(e GraphEdge) {
	if e.From == "" || e.To == "" {
		return
	}
	e.Exploitability = math.Max(0.05, math.Min(1.0, e.Exploitability))
	g.Adj[e.From] = append(g.Adj[e.From], e)
}

type PathBuildOptions struct {
	BaseMaxDepth     int
	HighRiskMaxDepth int
	BeamWidth        int
	MinStrength      float64
	MaxPathsPerType  int
	MaxTotalPaths    int
}

func defaultPathBuildOptions(opts PathBuildOptions) PathBuildOptions {
	if opts.BaseMaxDepth <= 0 {
		opts.BaseMaxDepth = 4
	}
	if opts.HighRiskMaxDepth <= 0 {
		opts.HighRiskMaxDepth = 5
	}
	if opts.BeamWidth <= 0 {
		opts.BeamWidth = 5
	}
	if opts.MinStrength <= 0 {
		opts.MinStrength = 0.15
	}
	if opts.MaxPathsPerType <= 0 {
		opts.MaxPathsPerType = 2
	}
	if opts.MaxTotalPaths <= 0 {
		opts.MaxTotalPaths = 6
	}
	return opts
}

// BuildTopPathsAdaptive generates production-oriented attack paths:
// adaptive depth + beam search + min strength pruning + top-k per class.
func (g *AttackGraph) BuildTopPathsAdaptive(
	startID string,
	targetTypes map[string]bool,
	highRiskSource bool,
	opts PathBuildOptions,
	feasible func(edge GraphEdge, from, to GraphNode) bool,
) []GraphPath {
	opts = defaultPathBuildOptions(opts)
	maxDepth := opts.BaseMaxDepth
	if highRiskSource {
		maxDepth = opts.HighRiskMaxDepth
	}
	start, ok := g.Nodes[startID]
	if !ok {
		return nil
	}

	type qItem struct {
		nodeIDs   []string
		edgeTypes []string
		strength  float64
		logSum    float64
		minEdge   float64
		visited   map[string]bool
	}
	queue := []qItem{{
		nodeIDs:   []string{start.ID},
		edgeTypes: []string{},
		strength:  1.0,
		logSum:    0.0,
		minEdge:   1.0,
		visited:   map[string]bool{start.ID: true},
	}}
	candidates := make([]GraphPath, 0, opts.MaxTotalPaths*2)

	for depth := 0; depth <= maxDepth && len(queue) > 0; depth++ {
		level := queue
		queue = nil
		next := make([]qItem, 0, len(level)*2)
		for _, cur := range level {
			lastID := cur.nodeIDs[len(cur.nodeIDs)-1]
			lastNode := g.Nodes[lastID]
			if targetTypes[lastNode.Type] && len(cur.edgeTypes) > 0 {
				pathLen := len(cur.edgeTypes)
				geometricMean := math.Exp(cur.logSum / float64(pathLen))
				minEdgeClamped := math.Max(cur.minEdge, 0.1)
				hybrid := 0.6*minEdgeClamped + 0.4*geometricMean
				penalty := math.Exp(-0.25 * float64(pathLen))
				directnessFactor := 1.0 / (1.0 + 0.2*float64(maxInt(pathLen-1, 0)))
				penalizedStrength := hybrid * penalty * directnessFactor
				uncertainEdges := countUncertainEdges(cur.edgeTypes)
				if uncertainEdges > 0 {
					uncertainPenalty := math.Exp(-0.35 * float64(uncertainEdges))
					penalizedStrength *= math.Max(0.3, uncertainPenalty)
				}
				penalizedStrength = math.Min(1.0, penalizedStrength)
				candidates = append(candidates, GraphPath{
					NodeIDs:   append([]string(nil), cur.nodeIDs...),
					EdgeTypes: append([]string(nil), cur.edgeTypes...),
					Strength:  math.Round(penalizedStrength*1000) / 1000,
					Class:     classifyPath(cur.edgeTypes, lastNode.Type, lastNode.ID),
				})
				// Early-stop at crown assets.
				if lastNode.Type == NodeTypeNode || lastNode.Namespace == "kube-system" {
					continue
				}
			}
			if len(cur.edgeTypes) >= maxDepth {
				continue
			}
			for _, e := range g.Adj[lastID] {
				toNode, exists := g.Nodes[e.To]
				if !exists {
					continue
				}
				if cur.visited[e.To] {
					continue
				}
				if feasible != nil && !feasible(e, lastNode, toNode) {
					continue
				}
				nextStrength := cur.strength * e.Exploitability
				if nextStrength < opts.MinStrength {
					continue
				}
				nextVisited := make(map[string]bool, len(cur.visited)+1)
				for k, v := range cur.visited {
					nextVisited[k] = v
				}
				nextVisited[e.To] = true
				next = append(next, qItem{
					nodeIDs:   append(append([]string(nil), cur.nodeIDs...), e.To),
					edgeTypes: append(append([]string(nil), cur.edgeTypes...), e.Type),
					strength:  nextStrength,
					logSum:    cur.logSum + math.Log(math.Max(0.05, e.Exploitability)),
					minEdge:   math.Min(cur.minEdge, e.Exploitability),
					visited:   nextVisited,
				})
			}
		}
		sort.SliceStable(next, func(i, j int) bool { return next[i].strength > next[j].strength })
		if len(next) > opts.BeamWidth {
			next = next[:opts.BeamWidth]
		}
		queue = next
	}

	return dedupeAndSelectPaths(candidates, opts)
}

func countUncertainEdges(edgeTypes []string) int {
	count := 0
	for _, et := range edgeTypes {
		if et == EdgeTypeNetworkReachSoft {
			count++
		}
	}
	return count
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func classifyPath(edgeTypes []string, endType string, endID string) string {
	for _, e := range edgeTypes {
		if e == EdgeTypeHostAccess || e == EdgeTypeContainerEscape {
			return PathClassEscape
		}
	}
	for _, e := range edgeTypes {
		if e == EdgeTypeNetworkReach || e == EdgeTypeLateralMove {
			return PathClassLateral
		}
	}
	if endType == NodeTypeClusterRole || endType == NodeTypeRole {
		if privilegeLevelFromID(endID) >= 3 {
			return PathClassPrivEsc
		}
		return PathClassDataExfil
	}
	return PathClassDataExfil
}

func dedupeAndSelectPaths(in []GraphPath, opts PathBuildOptions) []GraphPath {
	type key struct{ s, e, c string }
	best := map[key]GraphPath{}
	for _, p := range in {
		if len(p.NodeIDs) < 2 {
			continue
		}
		k := key{s: p.NodeIDs[0], e: p.NodeIDs[len(p.NodeIDs)-1], c: p.Class}
		if prev, ok := best[k]; !ok || p.Strength > prev.Strength {
			best[k] = p
		}
	}
	group := map[string][]GraphPath{}
	for _, p := range best {
		group[p.Class] = append(group[p.Class], p)
	}
	for cls := range group {
		sort.SliceStable(group[cls], func(i, j int) bool { return group[cls][i].Strength > group[cls][j].Strength })
		if len(group[cls]) > opts.MaxPathsPerType {
			group[cls] = group[cls][:opts.MaxPathsPerType]
		}
	}
	out := make([]GraphPath, 0, opts.MaxTotalPaths)
	for _, cls := range []string{PathClassEscape, PathClassPrivEsc, PathClassLateral, PathClassDataExfil} {
		out = append(out, group[cls]...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Strength > out[j].Strength
	})
	if len(out) > opts.MaxTotalPaths {
		out = out[:opts.MaxTotalPaths]
	}
	return out
}
