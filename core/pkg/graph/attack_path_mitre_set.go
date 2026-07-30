package graph

import (
	"encoding/json"
	"strings"

	"github.com/fortuna/core/pkg/models"
)

// MitreIDSetFromModels collects normalized ATT&CK IDs from attack_step nodes on persisted paths.
// Used to gate runtime MITRE boost so unrelated telemetry does not inflate path scores.
func MitreIDSetFromModels(rows []models.AttackPath) map[string]struct{} {
	out := make(map[string]struct{})
	for i := range rows {
		var nodes []PathNode
		if err := json.Unmarshal([]byte(rows[i].Nodes), &nodes); err != nil {
			continue
		}
		for _, n := range nodes {
			if !strings.EqualFold(strings.TrimSpace(n.Type), NodeTypeAttackStep) {
				continue
			}
			tid := strings.TrimSpace(n.ID)
			if tid == "" {
				continue
			}
			for _, ref := range MitreRefsForTechnique(tid) {
				mid := normalizeMitreIDForOverlay(ref.ID)
				if mid != "" {
					out[mid] = struct{}{}
				}
			}
		}
	}
	return out
}
