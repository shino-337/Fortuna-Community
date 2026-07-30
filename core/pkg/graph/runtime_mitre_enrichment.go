package graph

import (
	"context"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/k8scorroboration"
	"github.com/fortuna/core/pkg/models"
)

// When true, node-scoped steps only match events with non-empty node_name (stricter; can reduce coverage).
var enforceMitreScopeMatch = os.Getenv("FORTUNA_MITRE_ENFORCE_SCOPE") == "1"

// EnrichAttackChainsWithRuntimeMitre sets step-level runtime flags and per-chain mitre_summary
// by joining path step MITRE IDs to runtime_events for pods on the chain.
// No-op if db is nil, clusterID is empty, or runtime_events is missing.
func EnrichAttackChainsWithRuntimeMitre(db *gorm.DB, clusterID string, chains []AttackChain, paths []AttackPath) {
	if db == nil || clusterID == "" || !db.Migrator().HasTable("runtime_events") {
		return
	}
	pathsByID := make(map[string]AttackPath, len(paths))
	for _, p := range paths {
		if p.PathID != "" {
			pathsByID[p.PathID] = p
		}
	}
	if len(pathsByID) == 0 {
		return
	}
	since := time.Now().Add(-7 * 24 * time.Hour)
	for i := range chains {
		podUIDs := podUIDsInChainPaths(chains[i].Paths, pathsByID)
		eventRows := fetchRuntimeEventsDetailed(db, clusterID, podUIDs, since)
		rtByMitre := aggregateMitrePresence(eventRows)
		pathMitres := distinctPathMitreIDs(chains[i].Steps)
		matched := intersectStrings(pathMitres, rtByMitre)
		align := float64(len(matched)) / float64(mitreMaxDen(1, len(pathMitres)))

		evMatched := countEventsMatchingSteps(chains[i].Steps, eventRows)

		cp := float64(evMatched) / float64(mitreMaxDen(1, len(eventRows)))

		chains[i].MitreSummary = &AttackChainMitreSummary{
			PathMitreDistinct:       len(pathMitres),
			RuntimeMitreDistinct:    len(rtByMitre),
			MatchedMitreIDs:         matched,
			AlignmentRatio:          mathRound3(align),
			ObservingPodCount:       len(podUIDs),
			CorrelationPrecision:    mathRound3(cp),
			RuntimeEventsConsidered: len(eventRows),
			RuntimeEventsMatched:    evMatched,
		}
		for j := range chains[i].Steps {
			chains[i].Steps[j].RuntimeObserved = stepMatchesRuntime(&chains[i].Steps[j], eventRows)
			loose := countStepEventsLooseMitre(&chains[i].Steps[j], eventRows)
			strict := countStepEventsStrictMitreAndScope(&chains[i].Steps[j], eventRows)
			if loose > 0 {
				ratio := float64(strict) / float64(loose)
				rel := 1.0
				if loose < 3 {
					rel = 0.7
				}
				g := mathRound3(ratio * rel)
				chains[i].Steps[j].RuntimeGroundingScore = &g
			}
		}
		avgG := averageStepGrounding(chains[i].Steps)
		chains[i].MitreCoverage = BuildMitreCoverage(pathMitres, rtByMitre, chains[i].Realism, avgG)

		evFull := fetchRuntimeEventsForChainSatisfaction(db, clusterID, podUIDs, since)
		sigFull := fetchRuntimeSignalsForChainSatisfaction(db, clusterID, podUIDs, since)
		now := time.Now().UTC()
		saW := k8scorroboration.SATokenSatisfactionWeight(evFull, sigFull, now, k8scorroboration.DefaultSATokenSatisfactionHalfLife)
		satW := map[string]float64{}
		if saW > 0 {
			satW["SA_TOKEN"] = saW
		}
		chains[i].CapabilityValidation = BuildCapabilityValidationWithRuntimeWeights(chains[i].Steps, satW)
		if chains[i].RealismPreCapabilityValidation > 0 {
			chains[i].Realism = chains[i].RealismPreCapabilityValidation
		}
		applyCapabilityValidationToRealism(&chains[i])
	}
}

func fetchRuntimeEventsForChainSatisfaction(db *gorm.DB, clusterID string, podUIDs []string, since time.Time) []models.RuntimeEvent {
	if len(podUIDs) == 0 || db == nil {
		return nil
	}
	var rows []models.RuntimeEvent
	err := db.WithContext(context.Background()).
		Joins("INNER JOIN pods ON pods.uid = runtime_events.pod_uid AND pods.deleted_at IS NULL").
		Where("pods.cluster_id = ? AND runtime_events.pod_uid IN ? AND runtime_events.created_at > ? AND COALESCE(runtime_events.confidence, 0) > 0",
			clusterID, podUIDs, since).
		Find(&rows).Error
	if err != nil {
		return nil
	}
	return rows
}

func fetchRuntimeSignalsForChainSatisfaction(db *gorm.DB, clusterID string, podUIDs []string, since time.Time) []models.RuntimeSignal {
	if len(podUIDs) == 0 || db == nil {
		return nil
	}
	var rows []models.RuntimeSignal
	err := db.WithContext(context.Background()).
		Joins("INNER JOIN pods ON pods.uid = runtime_signals.pod_uid AND pods.deleted_at IS NULL").
		Where("pods.cluster_id = ? AND runtime_signals.pod_uid IN ? AND runtime_signals.created_at > ?",
			clusterID, podUIDs, since).
		Find(&rows).Error
	if err != nil {
		return nil
	}
	return rows
}

func fetchRuntimeEventsDetailed(db *gorm.DB, clusterID string, podUIDs []string, since time.Time) []runtimeEventLite {
	if len(podUIDs) == 0 {
		return nil
	}
	var rows []runtimeEventLite
	q := `
SELECT TRIM(UPPER(re.mitre_technique)) AS mitre_technique,
       re.created_at AS created_at,
       COALESCE(re.node_name, '') AS node_name,
       re.pod_uid AS pod_uid,
       COALESCE(p.namespace, '') AS pod_namespace
FROM runtime_events AS re
INNER JOIN pods AS p ON p.uid = re.pod_uid
WHERE p.cluster_id = ?
  AND p.deleted_at IS NULL
  AND re.pod_uid IN (?)
  AND TRIM(re.mitre_technique) <> ''
  AND re.created_at > ?
`
	if err := db.WithContext(context.Background()).Raw(q, clusterID, podUIDs, since).Scan(&rows).Error; err != nil {
		return nil
	}
	return rows
}

func aggregateMitrePresence(rows []runtimeEventLite) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		id := normalizeMitreID(r.Mitre)
		if id != "" {
			out[id] = true
		}
	}
	return out
}

func eventMitreMatchesStepMitres(ev runtimeEventLite, step *AttackStep) bool {
	em := normalizeMitreID(ev.Mitre)
	if em == "" {
		return false
	}
	for _, mt := range step.MitreTechniques {
		if normalizeMitreID(mt.ID) == em {
			return true
		}
	}
	return false
}

func stepMatchesRuntime(step *AttackStep, events []runtimeEventLite) bool {
	for _, ev := range events {
		if !eventMitreMatchesStepMitres(ev, step) {
			continue
		}
		if enforceMitreScopeMatch && scopeNeedsNode(step.ContextScopes) && strings.TrimSpace(ev.NodeName) == "" {
			continue
		}
		return true
	}
	return false
}

func scopeNeedsNode(scopes []string) bool {
	for _, s := range scopes {
		if strings.EqualFold(strings.TrimSpace(s), "node") {
			return true
		}
	}
	return false
}

// countStepEventsLooseMitre counts events whose MITRE id is on the step (denominator for step grounding).
func countStepEventsLooseMitre(step *AttackStep, events []runtimeEventLite) int {
	if len(events) == 0 {
		return 0
	}
	n := 0
	for _, ev := range events {
		if eventMitreMatchesStepMitres(ev, step) {
			n++
		}
	}
	return n
}

// countStepEventsStrictMitreAndScope counts events that also match step context_scope (attribution).
func countStepEventsStrictMitreAndScope(step *AttackStep, events []runtimeEventLite) int {
	if len(events) == 0 {
		return 0
	}
	n := 0
	for _, ev := range events {
		if !eventMitreMatchesStepMitres(ev, step) {
			continue
		}
		if len(step.ContextScopes) == 0 {
			n++
			continue
		}
		if eventMatchesContextScopesForStep(ev, step.ContextScopes) {
			n++
		}
	}
	return n
}

func eventMatchesContextScopesForStep(ev runtimeEventLite, scopes []string) bool {
	if len(scopes) == 0 {
		return true
	}
	return eventMatchesContextScopes(ev, scopes)
}

func countEventsMatchingSteps(steps []AttackStep, events []runtimeEventLite) int {
	if len(events) == 0 || len(steps) == 0 {
		return 0
	}
	n := 0
	for _, ev := range events {
		for i := range steps {
			if eventMitreMatchesStepMitres(ev, &steps[i]) {
				if enforceMitreScopeMatch && scopeNeedsNode(steps[i].ContextScopes) && strings.TrimSpace(ev.NodeName) == "" {
					continue
				}
				n++
				break
			}
		}
	}
	return n
}

func podUIDsInChainPaths(pathIDs []string, pathsByID map[string]AttackPath) []string {
	seen := map[string]bool{}
	var out []string
	for _, pid := range pathIDs {
		p, ok := pathsByID[pid]
		if !ok || len(p.Nodes) == 0 {
			continue
		}
		first := p.Nodes[0]
		if strings.EqualFold(strings.TrimSpace(first.Type), NodeTypePod) && first.ID != "" && !seen[first.ID] {
			seen[first.ID] = true
			out = append(out, first.ID)
		}
	}
	return out
}

func distinctPathMitreIDs(steps []AttackStep) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range steps {
		for _, m := range s.MitreTechniques {
			id := normalizeMitreID(m.ID)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func intersectStrings(pathIDs []string, rt map[string]bool) []string {
	var out []string
	for _, id := range pathIDs {
		n := normalizeMitreID(id)
		if n != "" && rt[n] {
			out = append(out, n)
		}
	}
	return out
}

func normalizeMitreID(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func mitreMaxDen(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mathRound3(x float64) float64 {
	return float64(int(x*1000+0.5)) / 1000
}

func averageStepGrounding(steps []AttackStep) float64 {
	var sum float64
	n := 0
	for _, s := range steps {
		if s.RuntimeGroundingScore != nil {
			sum += *s.RuntimeGroundingScore
			n++
		}
	}
	if n == 0 {
		return 1.0
	}
	return sum / float64(n)
}
