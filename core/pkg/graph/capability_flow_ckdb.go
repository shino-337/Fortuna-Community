package graph

import (
	"encoding/json"
	"strings"

	"github.com/fortuna/core/pkg/models"
)

// CapabilityFlowSetFromAttackPaths derives capability tokens that appear on persisted attack-path
// topology (capability nodes + Requires/Provides from attack_step technique nodes).
func CapabilityFlowSetFromAttackPaths(paths []models.AttackPath) map[string]bool {
	out := map[string]bool{}
	for _, ap := range paths {
		if strings.TrimSpace(ap.Nodes) == "" || ap.Nodes == "[]" {
			continue
		}
		var nodes []PathNode
		if err := json.Unmarshal([]byte(ap.Nodes), &nodes); err != nil {
			continue
		}
		for _, n := range nodes {
			switch strings.ToLower(strings.TrimSpace(n.Type)) {
			case NodeTypeCapability:
				raw := strings.ToUpper(strings.TrimSpace(n.ID))
				if raw != "" {
					out[raw] = true
					out[strings.ToUpper(CanonicalCapability(raw))] = true
				}
			case NodeTypeAttackStep:
				tid := strings.ToUpper(strings.TrimSpace(n.ID))
				t, ok := TechniqueByID(tid)
				if !ok {
					continue
				}
				for _, x := range t.Requires {
					out[strings.ToUpper(CanonicalCapability(x))] = true
				}
				for _, x := range t.Provides {
					out[strings.ToUpper(CanonicalCapability(x))] = true
				}
			}
		}
	}
	return out
}

// FilterPodCapabilitiesForCKDB keeps pod capability rows whose IDs appear on at least one
// persisted attack path’s capability flow. When flow is empty (no paths / parse failure),
// returns all distinct capability IDs from caps (backward compatible).
func FilterPodCapabilitiesForCKDB(paths []models.AttackPath, caps []models.PodCapability) []string {
	flow := CapabilityFlowSetFromAttackPaths(paths)
	seen := map[string]bool{}
	var out []string
	for _, c := range caps {
		raw := strings.TrimSpace(c.CapabilityID)
		if raw == "" || seen[raw] {
			continue
		}
		can := strings.ToUpper(CanonicalCapability(raw))
		up := strings.ToUpper(raw)
		if len(flow) > 0 {
			if !flow[up] && !flow[can] {
				continue
			}
		}
		seen[raw] = true
		out = append(out, raw)
	}
	return out
}
