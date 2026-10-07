package graph

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
)

// StablePathID identifies a path by what it contains (its nodes and edges), so the
// same path keeps the same id across recomputations, clusters and endpoints. Alerts,
// cases and deep links store it; a positional id (p0, p1, …) changes whenever pod
// or cluster iteration order changes.
func StablePathID(p AttackPath) string {
	nodes := make([]string, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		nodes = append(nodes, n.Type+"\x1f"+n.ID)
	}
	edges := make([]string, 0, len(p.Edges))
	for _, e := range p.Edges {
		edges = append(edges, e.Type+"\x1f"+e.Source+"\x1f"+e.Target)
	}
	sort.Strings(nodes)
	sort.Strings(edges)

	h := sha1.New()
	// The entry pod leads the key so two paths over the same objects from different pods differ.
	if len(p.Nodes) > 0 {
		h.Write([]byte(p.Nodes[0].ID))
	}
	h.Write([]byte{0x1d})
	for _, n := range nodes {
		h.Write([]byte(n))
		h.Write([]byte{0x1e})
	}
	h.Write([]byte{0x1d})
	for _, e := range edges {
		h.Write([]byte(e))
		h.Write([]byte{0x1e})
	}
	return "ap_" + hex.EncodeToString(h.Sum(nil))[:16]
}

// StablePathIDs returns StablePathID for each path; exact duplicates get a -2, -3 … suffix.
func StablePathIDs(paths []AttackPath) []string {
	out := make([]string, len(paths))
	seen := make(map[string]int, len(paths))
	for i := range paths {
		id := StablePathID(paths[i])
		seen[id]++
		if n := seen[id]; n > 1 {
			id = fmt.Sprintf("%s-%d", id, n)
		}
		out[i] = id
	}
	return out
}

// AssignStablePathIDs sets PathID on every path to its StablePathIDs value.
func AssignStablePathIDs(paths []AttackPath) {
	for i, id := range StablePathIDs(paths) {
		paths[i].PathID = id
	}
}
