package explainability

import (
	"context"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// EnrichedFactRef is a minimal fact row for insight drill-down (G-EXP-01).
type EnrichedFactRef struct {
	FactID     string `json:"factId"`
	FactType   string `json:"factType,omitempty"`
	Domain     string `json:"domain,omitempty"`
	ObservedAt string `json:"observedAt,omitempty"`
}

// EnrichedRefs attaches resolved DB rows for IDs already listed in evidence_refs.
type EnrichedRefs struct {
	Facts []EnrichedFactRef `json:"facts,omitempty"`
}

// LoadFactSummaries loads runtime_behavior_facts for the given pod and fact_id list (best-effort).
func LoadFactSummaries(ctx context.Context, db *gorm.DB, podUID string, factIDs []string) ([]EnrichedFactRef, error) {
	if db == nil || strings.TrimSpace(podUID) == "" || len(factIDs) == 0 {
		return nil, nil
	}
	clean := make([]string, 0, len(factIDs))
	seen := map[string]struct{}{}
	for _, id := range factIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return nil, nil
	}
	var rows []models.RuntimeBehaviorFact
	if err := db.WithContext(ctx).
		Where("pod_uid = ? AND fact_id IN ?", podUID, clean).
		Order("observed_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]EnrichedFactRef, 0, len(rows))
	for i := range rows {
		r := EnrichedFactRef{
			FactID:   rows[i].FactID,
			FactType: rows[i].FactType,
			Domain:   rows[i].Domain,
		}
		if !rows[i].ObservedAt.IsZero() {
			r.ObservedAt = rows[i].ObservedAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		out = append(out, r)
	}
	return out, nil
}

// BuildEnrichedRefs returns a bundle suitable for JSON embedding on GET /risk/insights/:id?enrich=1.
func BuildEnrichedRefs(ctx context.Context, db *gorm.DB, podUID string, factIDs []string) (*EnrichedRefs, error) {
	facts, err := LoadFactSummaries(ctx, db, podUID, factIDs)
	if err != nil {
		return nil, err
	}
	if len(facts) == 0 {
		return nil, nil
	}
	return &EnrichedRefs{Facts: facts}, nil
}
