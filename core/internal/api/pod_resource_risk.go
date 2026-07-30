package api

import (
	"context"
	"sort"
	"strings"

	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
	"gorm.io/gorm"
)

const maxPodsForAttackPathSort = 5000

type podV3ScoreRec struct {
	ResourceUID string  `gorm:"column:resource_uid"`
	ClusterID   string  `gorm:"column:cluster_id"`
	TotalScore  float64 `gorm:"column:total_score"`
}

// loadLatestV3RiskScoresByPod returns first (latest) v3 row per resource_uid+cluster_id.
func loadLatestV3RiskScoresByPod(db *gorm.DB, pods []models.Pod) map[string]podV3ScoreRec {
	out := make(map[string]podV3ScoreRec)
	if len(pods) == 0 || !hasTable(db, "risk_scores") {
		return out
	}
	seen := make(map[string]struct{}, len(pods))
	uids := make([]string, 0, len(pods))
	for _, p := range pods {
		if _, ok := seen[p.UID]; ok {
			continue
		}
		seen[p.UID] = struct{}{}
		uids = append(uids, p.UID)
	}
	var rows []podV3ScoreRec
	if err := db.Model(&models.RiskScore{}).
		Select("resource_uid, cluster_id, total_score").
		Where("resource_uid IN ? AND LOWER(resource_type) = 'pod' AND deleted_at IS NULL AND LOWER(TRIM(COALESCE(scorer_version, ''))) = ?", uids, "v3").
		Order("calculated_at DESC, id DESC").
		Find(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		key := r.ResourceUID + "|" + r.ClusterID
		if _, exists := out[key]; !exists {
			out[key] = r
		}
	}
	return out
}

type clusterChainsCacheEntry struct {
	chains   []graph.AttackChain
	pathByID map[string]graph.AttackPath
}

func getAttackChainsCached(ctx context.Context, db *gorm.DB, cache map[string]*clusterChainsCacheEntry, clusterID string) *clusterChainsCacheEntry {
	if clusterID == "" {
		return nil
	}
	if e, ok := cache[clusterID]; ok {
		return e
	}
	builder := graph.NewRelationalPathBuilder(db)
	chains, pathByID, err := builder.GetChainsWithPaths(ctx, clusterID)
	if err != nil || chains == nil {
		chains = []graph.AttackChain{}
	}
	if pathByID == nil {
		pathByID = map[string]graph.AttackPath{}
	}
	e := &clusterChainsCacheEntry{chains: chains, pathByID: pathByID}
	cache[clusterID] = e
	return e
}

// loadPersistedAttackPathSummariesByPodUID returns attack_paths rows (description + strength) keyed by pod_uid for signal merge.
func loadPersistedAttackPathSummariesByPodUID(db *gorm.DB, podUIDs []string) map[string][]graph.PersistedAttackPathSummary {
	out := make(map[string][]graph.PersistedAttackPathSummary)
	if db == nil || len(podUIDs) == 0 || !hasTable(db, "attack_paths") {
		return out
	}
	seen := make(map[string]struct{}, len(podUIDs))
	uids := make([]string, 0, len(podUIDs))
	for _, u := range podUIDs {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		uids = append(uids, u)
	}
	var rows []models.AttackPath
	if err := db.Model(&models.AttackPath{}).
		Select("pod_uid, path_id, description, total_risk").
		Where("pod_uid IN ?", uids).
		Find(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		uid := strings.TrimSpace(r.PodUID)
		if uid == "" {
			continue
		}
		out[uid] = append(out[uid], graph.PersistedAttackPathSummary{
			PathID:      r.PathID,
			Description: r.Description,
			TotalRisk:   r.TotalRisk,
		})
	}
	return out
}

// podResourceRiskSignalsWithScore builds graph→resource signals for a single pod (pod detail parity with list rows).
func podResourceRiskSignalsWithScore(ctx context.Context, db *gorm.DB, pod models.Pod, unifiedTotal float64) graph.ResourceRiskSignals {
	chainCache := make(map[string]*clusterChainsCacheEntry)
	bundle := getAttackChainsCached(ctx, db, chainCache, pod.ClusterID)
	var chains []graph.AttackChain
	var pathByID map[string]graph.AttackPath
	if bundle != nil {
		chains = bundle.chains
		pathByID = bundle.pathByID
	}
	persisted := loadPersistedAttackPathSummariesByPodUID(db, []string{pod.UID})
	return graph.BuildResourceRiskSignals(pod.UID, unifiedTotal, chains, pathByID, persisted[pod.UID])
}

type podRowSortable struct {
	models.Pod
	RiskCount      int64                     `json:"riskCount"`
	UnifiedScore   *float64                  `json:"unifiedScore,omitempty"`
	TotalScore     *float64                  `json:"total_score,omitempty"`
	FinalLevel     string                    `json:"finalLevel,omitempty"`
	ScorerVersion  string                    `json:"scorerVersion,omitempty"`
	RiskSignals    graph.ResourceRiskSignals `json:"risk_signals"`
	// PathPreview is a short human-readable chain hint for table rows (persisted description or step names).
	PathPreview string `json:"path_preview,omitempty"`
	// BlastEntityCount = distinct graph node IDs on touching chains (workloads + identities + …).
	BlastEntityCount int `json:"blast_entity_count,omitempty"`
	// RiskFixHintPct is an advisory “up to ~X% reduction if remediated” heuristic from unified score (not a guarantee).
	RiskFixHintPct int `json:"risk_fix_hint_pct,omitempty"`
	unifiedSortVal float64                   `json:"-"`
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func buildPathPreviewAndBlast(uid string, chains []graph.AttackChain, persisted []graph.PersistedAttackPathSummary) (preview string, blastCount int) {
	touching := graph.ChainsTouchingResource(uid, chains)
	seen := make(map[string]struct{})
	for i := range touching {
		for _, id := range touching[i].InvolvedResources {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			seen[id] = struct{}{}
		}
	}
	blastCount = len(seen)

	for _, ps := range persisted {
		d := strings.TrimSpace(ps.Description)
		if d != "" {
			preview = truncateRunes(d, 120)
			return
		}
	}
	if len(touching) == 0 {
		return "", blastCount
	}
	ch := touching[0]
	var parts []string
	for _, st := range ch.Steps {
		if len(parts) >= 6 {
			break
		}
		n := strings.TrimSpace(st.Name)
		if n == "" {
			n = strings.TrimSpace(st.TechniqueID)
		}
		if n != "" {
			parts = append(parts, n)
		}
	}
	if len(parts) > 0 {
		preview = strings.Join(parts, " → ")
		preview = truncateRunes(preview, 120)
		return
	}
	if h := strings.TrimSpace(ch.Story.Headline); h != "" {
		preview = truncateRunes(h, 120)
	}
	return
}

func riskFixHintPct(score float64) int {
	if score >= 85 {
		return 45
	}
	if score >= 70 {
		return 35
	}
	if score >= 50 {
		return 25
	}
	if score >= 30 {
		return 15
	}
	return 10
}

func sortPodRowsByAttackPathPriority(rows []podRowSortable) {
	sort.SliceStable(rows, func(i, j int) bool {
		ha, ia, ea, sa := graph.AttackPathSortPriority(rows[i].RiskSignals, rows[i].unifiedSortVal)
		hb, ib, eb, sb := graph.AttackPathSortPriority(rows[j].RiskSignals, rows[j].unifiedSortVal)
		if ha != hb {
			return ha > hb
		}
		if ia != ib {
			return ia > ib
		}
		if ea != eb {
			return ea > eb
		}
		if sa != sb {
			return sa > sb
		}
		return rows[i].Name < rows[j].Name
	})
}

func buildPodRowsWithRiskSignals(ctx context.Context, db *gorm.DB, pods []models.Pod, riskByUID map[string]int64, scores map[string]podV3ScoreRec, chainCache map[string]*clusterChainsCacheEntry) []podRowSortable {
	uidsForPaths := make([]string, 0, len(pods))
	for _, p := range pods {
		uidsForPaths = append(uidsForPaths, p.UID)
	}
	persistedByPod := loadPersistedAttackPathSummariesByPodUID(db, uidsForPaths)

	out := make([]podRowSortable, 0, len(pods))
	for _, p := range pods {
		bundle := getAttackChainsCached(ctx, db, chainCache, p.ClusterID)
		var chains []graph.AttackChain
		var pathByID map[string]graph.AttackPath
		if bundle != nil {
			chains = bundle.chains
			pathByID = bundle.pathByID
		}
		var us *float64
		var final string
		sv := ""
		uval := 0.0
		if rec, ok := scores[p.UID+"|"+p.ClusterID]; ok {
			ts := rec.TotalScore
			us = &ts
			final = risk.DeriveFinalLevelFromScore(ts)
			sv = "v3"
			uval = ts
		}
		sig := graph.BuildResourceRiskSignals(p.UID, uval, chains, pathByID, persistedByPod[p.UID])
		pathPreview, blastN := buildPathPreviewAndBlast(p.UID, chains, persistedByPod[p.UID])
		fixPct := 0
		if us != nil {
			fixPct = riskFixHintPct(*us)
		}
		row := podRowSortable{
			Pod:              p,
			RiskCount:        riskByUID[p.UID],
			UnifiedScore:     us,
			FinalLevel:       final,
			ScorerVersion:    sv,
			RiskSignals:      sig,
			PathPreview:      pathPreview,
			BlastEntityCount: blastN,
			RiskFixHintPct:   fixPct,
			unifiedSortVal:   uval,
		}
		if us != nil {
			ts := *us
			row.TotalScore = &ts
		}
		out = append(out, row)
	}
	return out
}
