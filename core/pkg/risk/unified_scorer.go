// Package risk provides Fortuna's unified risk scoring pipeline.
//
// unified_scorer.go implements the Unified Scorer V3 that reads from ALL
// pipeline sources and produces a single authoritative 0–100 risk score per
// resource:
//
//	Layer 1 (insights + pod_capabilities)   → 7 weighted dimensions
//	Layer 2 (runtime_signals/incidents)     → RUNTIME_THREAT dimension
//	Layer 3 (attack_paths)                  → ATTACK_PATH + BLAST_RADIUS dimensions
//	Layer 4 (this scorer)                   → unified risk_scores record
//
// V3 is the only scorer persisted and consumed by Core HTTP/sync paths.
package risk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/k8scorroboration"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
)

// --- dimension constants -------------------------------------------------

// Maximum points per dimension (sum = 90; remaining 10 = blast radius)
const (
	dimMaxVulnerability      = 15.0
	dimMaxCapabilityExposure = 15.0
	dimMaxAttackPath         = 15.0
	dimMaxRBACPolicy         = 15.0
	dimMaxRuntimeThreat      = 15.0
	dimMaxExposure           = 15.0
	dimMaxBlastRadius        = 10.0
)

// System namespaces receive a reduced context multiplier — expected to have
// elevated capabilities that don't represent the same risk as workload pods.
var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

// UnifiedScorerV3 computes risk scores from all pipeline layers.
type UnifiedScorerV3 struct {
	db *gorm.DB
}

// NewUnifiedScorerV3 creates a new V3 scorer.
func NewUnifiedScorerV3(db *gorm.DB) *UnifiedScorerV3 {
	return &UnifiedScorerV3{db: db}
}

// UnifiedScoreV3 is the result produced by the V3 scorer.
type UnifiedScoreV3 struct {
	ResourceUID   string
	ResourceName  string
	ResourceType  string
	Namespace     string
	ClusterID     string
	PriorityLevel string
	TotalScore    float64
	TimeDecay     float64
	ScorerVersion string

	// Dimension sub-scores
	VulnerabilityScore      float64
	CapabilityExposureScore float64
	AttackPathScore         float64
	RBACPolicyScore         float64
	RuntimeThreatScore      float64
	ExposureScore           float64
	BlastRadiusScore        float64

	// Toxic combos triggered
	ToxicCombos []string

	// Stored as JSON in risk_scores.factors
	Factors map[string]interface{}

	CalculatedAt time.Time
}

// pathRuntimeAlign carries precomputed runtime↔attack-step alignment for V3 scoring
// so path influence and the runtime dimension share one inference pass.
type pathRuntimeAlign struct {
	Progress         float64
	MatchedSignals   []string
	CriticalHitCount int
}

type pathInfluence struct {
	MaxStrength           float64
	AvgStrength           float64
	Progress              float64
	ReachesNode           bool
	ClassCounts           map[string]int
	EInjectNorm           float64
	RInjectNorm           float64
	IInjectNorm           float64
	EInjectPoints         float64
	RInjectPoints         float64
	IInjectPoints         float64
	ProgressRaw           float64
	ProgressSaturated     float64
	InjectScale           float64
	GraphBasedE           float64
	RuntimeBasedE         float64
	RuntimeDecay          float64
	DiversityFactor       float64
	CriticalHitCount      int
	AppliedPathCount      int
	RuntimeStepProgress   float64
	MatchedRuntimeSignals []string
}

// CalculateScoreV3 is a fail-closed compatibility wrapper. Production
// callers should pass an explicit canonical identity to CalculateScoreV3ForIdentity.
func (s *UnifiedScorerV3) CalculateScoreV3(ctx context.Context, resourceUID string) (*UnifiedScoreV3, error) {
	if strings.TrimSpace(resourceUID) == "" {
		return nil, fmt.Errorf("resourceUID must not be empty")
	}
	var owners []string
	if err := s.db.WithContext(ctx).Model(&models.Pod{}).
		Where("uid = ? AND deleted_at IS NULL", resourceUID).
		Distinct().Order("cluster_id").Pluck("cluster_id", &owners).Error; err != nil {
		return nil, err
	}
	if len(owners) != 1 {
		return nil, fmt.Errorf("cluster-qualified pod identity required: uid=%s owners=%d", resourceUID, len(owners))
	}
	id, err := resourceidentity.New(owners[0], resourceUID)
	if err != nil {
		return nil, err
	}
	return s.CalculateScoreV3ForIdentity(ctx, id)
}

// SaveScoreV3 persists a V3 score to the risk_scores table.
// It upserts by resource_uid so each resource has at most one V3 row.
func (s *UnifiedScorerV3) SaveScoreV3(ctx context.Context, score *UnifiedScoreV3) error {
	factorsJSON, _ := json.Marshal(score.Factors)

	record := models.RiskScore{
		ResourceType:            score.ResourceType,
		ResourceUID:             score.ResourceUID,
		ResourceName:            score.ResourceName,
		Namespace:               score.Namespace,
		ClusterID:               score.ClusterID,
		TotalScore:              score.TotalScore,
		BaseScore:               score.VulnerabilityScore + score.RBACPolicyScore,
		ExploitabilityScore:     score.CapabilityExposureScore + score.AttackPathScore,
		BusinessImpactScore:     score.RuntimeThreatScore + score.ExposureScore,
		TimeDecay:               score.TimeDecay,
		ScorerVersion:           "v3",
		Factors:                 string(factorsJSON),
		PriorityLevel:           score.PriorityLevel,
		CalculatedAt:            score.CalculatedAt,
		CapabilityExposureScore: score.CapabilityExposureScore,
		AttackPathScore:         score.AttackPathScore,
		RuntimeThreatScore:      score.RuntimeThreatScore,
		BlastRadiusScore:        score.BlastRadiusScore,
	}

	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			// Keep conflict target aligned with risk_scores UNIQUE constraint:
			// (resource_type, resource_uid, cluster_id).
			Columns: []clause.Column{
				{Name: "resource_type"},
				{Name: "resource_uid"},
				{Name: "cluster_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"total_score", "base_score", "exploitability_score", "business_impact_score",
				"time_decay", "scorer_version", "factors", "priority_level", "calculated_at",
				"capability_exposure_score", "attack_path_score", "runtime_threat_score", "blast_radius_score",
				"resource_name", "namespace", "cluster_id", "updated_at", "deleted_at",
			}),
		}).
		Create(&record).Error
}

// ScheduleUnifiedScoreCalculation triggers an async V3 score calculation for the
// given resource UID.  It is a fire-and-forget goroutine — failures are logged but
// do not propagate.
func (s *UnifiedScorerV3) ScheduleUnifiedScoreCalculation(resourceUID string) {
	if resourceUID == "" {
		return
	}
	go func() {
		ctx := context.Background()
		score, err := s.CalculateScoreV3(ctx, resourceUID)
		if err != nil {
			log.Printf("[UnifiedScorerV3] calculation failed for %s: %v", resourceUID, err)
			return
		}
		if err := s.SaveScoreV3(ctx, score); err != nil {
			log.Printf("[UnifiedScorerV3] save failed for %s: %v", resourceUID, err)
			return
		}
		log.Printf("[UnifiedScorerV3] score updated for %s total=%.1f", resourceUID, score.TotalScore)
	}()
}

// --- dimension scorers ---------------------------------------------------

// scoreVulnerability scores up to dimMaxVulnerability based on CVE insights.
// It rewards the worst CVSS severity and exploit count diversity.
func (s *UnifiedScorerV3) scoreVulnerability(insights []models.Insight) float64 {
	var score float64
	for _, ins := range insights {
		if !strings.EqualFold(ins.InsightType, "vulnerability") {
			continue
		}
		switch strings.ToLower(ins.Severity) {
		case "critical":
			score = math.Max(score, 12.0)
		case "high":
			score = math.Max(score, 9.0)
		case "medium":
			score = math.Max(score, 5.0)
		case "low":
			score = math.Max(score, 2.0)
		}
		// High CVSS bonus (>= 9.0 critical threshold)
		if ins.CVSS >= 9.0 {
			score = math.Min(dimMaxVulnerability, score+3.0)
		}
	}
	return math.Min(dimMaxVulnerability, score)
}

// scoreCapabilityExposure scores up to dimMaxCapabilityExposure based on PCE data.
func (s *UnifiedScorerV3) scoreCapabilityExposure(caps []models.PodCapability) float64 {
	if len(caps) == 0 {
		return 0
	}

	severityMax := map[string]float64{
		"critical": 10.0,
		"high":     7.0,
		"medium":   4.0,
		"low":      1.0,
	}

	var maxSev, exploitedBonus, diversityBonus float64
	exploitedCount := 0
	for _, c := range caps {
		if v, ok := severityMax[strings.ToLower(c.Severity)]; ok {
			if v > maxSev {
				maxSev = v
			}
		}
		if c.State == "exploited" || c.State == "chained" {
			exploitedCount++
		}
	}
	exploitedBonus = math.Min(3.0, float64(exploitedCount)*1.5)
	diversityBonus = math.Min(2.0, float64(len(caps))*0.4)

	return math.Min(dimMaxCapabilityExposure, maxSev+exploitedBonus+diversityBonus)
}

// scoreAttackPathDim scores up to dimMaxAttackPath based on persisted paths.
// No synthetic floor is applied; Layer 3 materialized paths are the source of truth.
func (s *UnifiedScorerV3) scoreAttackPathDim(paths []models.AttackPath) float64 {
	var maxRisk float64
	criticalCount := 0
	for _, p := range paths {
		if p.TotalRisk > maxRisk {
			maxRisk = p.TotalRisk
		}
		if p.TotalRisk >= 9.0 {
			criticalCount++
		}
	}

	// Scale maxRisk (0-10) to dimension (0-15)
	base := maxRisk * (dimMaxAttackPath / 10.0)
	criticalBonus := math.Min(3.0, float64(criticalCount)*1.0)

	// Check if any path reaches cluster-admin
	if hasClusterAdminAttackPath(paths) {
		criticalBonus = math.Max(criticalBonus, 3.0)
	}

	score := base + criticalBonus
	return math.Min(dimMaxAttackPath, score)
}

// scoreRBACPolicy scores up to dimMaxRBACPolicy based on RBAC/pod-security insights.
func (s *UnifiedScorerV3) scoreRBACPolicy(insights []models.Insight) float64 {
	var score float64
	for _, ins := range insights {
		t := strings.ToLower(ins.InsightType)
		if t != "rbac" && t != "pod-security" {
			continue
		}
		switch strings.ToLower(ins.Severity) {
		case "critical":
			score = math.Max(score, 13.0)
		case "high":
			score = math.Max(score, 10.0)
		case "medium":
			score = math.Max(score, 6.0)
		case "low":
			score = math.Max(score, 2.0)
		}
	}
	return math.Min(dimMaxRBACPolicy, score)
}

func filterRuntimeEventsScoringConfidence(in []models.RuntimeEvent) []models.RuntimeEvent {
	out := make([]models.RuntimeEvent, 0, len(in))
	for i := range in {
		if in[i].Confidence > 0 {
			out = append(out, in[i])
		}
	}
	return out
}

func runtimePathInfluenceMultiplier(runtimeScore, alignProgress, temporalCoherence float64) float64 {
	r := runtimeScore / dimMaxRuntimeThreat
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	a := clamp(alignProgress, 0, 1)
	t := temporalCoherence
	if t < 0.82 {
		t = 0.82
	}
	if t > 1.15 {
		t = 1.15
	}
	raw := r * (0.4 + 0.6*a) * t
	return clamp(raw, 0, 0.26)
}

func meanRuntimeDerivedCapabilityConfidence(caps []models.PodCapability) float64 {
	var sum float64
	n := 0
	for _, c := range caps {
		if strings.EqualFold(c.CapabilityGroup, "runtime_derived") {
			sum += c.Confidence
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func meanWeightedRuntimeSignalConfidence(signals []models.RuntimeSignal) float64 {
	var sum float64
	var w int64
	for _, rs := range signals {
		c := int64(rs.Count)
		if c < 1 {
			c = 1
		}
		sum += effectiveSignalConfidence(rs) * float64(c)
		w += c
	}
	if w == 0 {
		return 0
	}
	return sum / float64(w)
}

// computeRuntimeThreatWithMeta scores up to dimMaxRuntimeThreat (spec VIII).
func computeRuntimeThreatWithMeta(insights []models.Insight, signalEvents24h int64, runtimeSignals []models.RuntimeSignal, pathAlignProgress float64, events []models.RuntimeEvent, temporal RuntimeTemporalMeta, conflictMul float64) (float64, map[string]interface{}) {
	meta := map[string]interface{}{}
	if conflictMul <= 0 {
		conflictMul = 1
	}
	var insightScore float64
	for _, ins := range insights {
		t := strings.ToLower(ins.InsightType)
		if t != "runtime" && t != "runtime_threat" && t != "runtime-behavior" {
			continue
		}
		switch strings.ToLower(ins.Severity) {
		case "critical":
			insightScore = math.Max(insightScore, 12.0)
		case "high":
			insightScore = math.Max(insightScore, 9.0)
		case "medium":
			insightScore = math.Max(insightScore, 5.0)
		case "low":
			insightScore = math.Max(insightScore, 2.0)
		}
	}

	var shellEventWeight, netEventWeight int64
	for _, rs := range runtimeSignals {
		c := int64(rs.Count)
		if c < 1 {
			c = 1
		}
		st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
		cat := strings.ToLower(strings.TrimSpace(rs.Category))
		switch st {
		case "NETWORK_QUEUE_ANOMALY":
			netEventWeight += c
		case "INTERACTIVE_SHELL_EXEC", "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "EBPF_EXEC_ACTIVITY",
			"NAMESPACE_ESCAPE", "PROC_ROOT_PIVOT", "FS_ESCAPE_ATTEMPT", "CAPABILITY_MISUSE":
			shellEventWeight += c
		default:
			if cat == "network" {
				netEventWeight += c
			} else {
				shellEventWeight += c
			}
		}
	}
	if shellEventWeight == 0 && netEventWeight == 0 && signalEvents24h > 0 {
		// Fallback when aggregates omit per-signal counts: spread total across classes.
		shellEventWeight = signalEvents24h / 2
		netEventWeight = signalEvents24h - shellEventWeight
	}
	velocityShell := math.Min(2.5, math.Log1p(float64(shellEventWeight))*1.0)
	velocityNet := math.Min(2.0, math.Log1p(float64(netEventWeight))*1.0)
	velocityBonus := math.Min(5.0, velocityShell+velocityNet)
	meta["runtime_threat_velocity_shell_bonus"] = math.Round(velocityShell*100) / 100
	meta["runtime_threat_velocity_network_bonus"] = math.Round(velocityNet*100) / 100
	meta["runtime_threat_velocity_bonus"] = math.Round(velocityBonus*100) / 100

	distinctTypes := 0
	seen := map[string]bool{}
	for _, rs := range runtimeSignals {
		st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
		if st == "" {
			continue
		}
		if !seen[st] {
			seen[st] = true
			distinctTypes++
		}
	}
	diversityBonus := math.Min(4.0, float64(distinctTypes)*0.8)
	meta["runtime_signal_distinct_24h"] = distinctTypes
	meta["runtime_threat_diversity_bonus"] = math.Round(diversityBonus*100) / 100

	totalEvents := int64(0)
	for _, rs := range runtimeSignals {
		c := int64(rs.Count)
		if c < 1 {
			c = 1
		}
		totalEvents += c
	}
	entropyRatio := 0.0
	if totalEvents > 0 && distinctTypes > 0 {
		entropyRatio = float64(distinctTypes) / float64(totalEvents)
	}
	entropyBonus := math.Min(2.0, entropyRatio*2.0)
	meta["runtime_signal_entropy_ratio"] = math.Round(entropyRatio*1000) / 1000
	meta["runtime_threat_entropy_bonus"] = math.Round(entropyBonus*100) / 100

	alignProgress := clamp(pathAlignProgress, 0, 1)
	alignBonus := alignProgress * 1.5
	meta["runtime_path_align_progress"] = alignProgress
	meta["runtime_path_align_bonus"] = math.Round(alignBonus*100) / 100

	toxicBonus := 0.0
	if hasRuntimeInteractiveShellSignals(runtimeSignals) && CorroboratesK8sAPIAccess(events, runtimeSignals) {
		toxicBonus = 4.5
	}
	meta["runtime_toxic_combo_shell_api_bonus"] = toxicBonus
	meta["runtime_k8s_api_corroborated"] = CorroboratesK8sAPIAccess(events, runtimeSignals)

	meta["runtime_temporal_sequence_bonus"] = temporal.SequenceBonus
	meta["runtime_temporal_disorder_penalty"] = temporal.DisorderPenalty
	meta["runtime_temporal_coherence_multiplier"] = temporal.CoherenceMultiplier
	meta["runtime_conflict_multiplier"] = conflictMul

	runtimePart := velocityBonus + diversityBonus + entropyBonus + alignBonus + toxicBonus
	runtimePart *= conflictMul
	if temporal.CoherenceMultiplier > 0 {
		runtimePart *= temporal.CoherenceMultiplier
	}
	score := insightScore + runtimePart
	score = math.Min(dimMaxRuntimeThreat, score)
	return score, meta
}

// scoreExposureDim scores up to dimMaxExposure based on pod/resource metadata.
func (s *UnifiedScorerV3) scoreExposureDim(pod models.Pod, podFound bool) float64 {
	if !podFound {
		return 0
	}
	var score float64
	if pod.HostNetwork {
		score += 8.0
	}
	if pod.HostPID {
		score += 5.0
	}
	if pod.HostIPC {
		score += 4.0
	}
	return math.Min(dimMaxExposure, score)
}

// scoreBlastRadiusDim scores up to dimMaxBlastRadius.
func (s *UnifiedScorerV3) scoreBlastRadiusDim(paths []models.AttackPath, caps []models.PodCapability) float64 {
	var score float64
	// Cluster-admin path = potential cluster-wide blast radius
	if hasClusterAdminAttackPath(paths) {
		score = math.Max(score, 8.0)
	} else if len(paths) > 0 {
		score = math.Max(score, 4.0)
	}
	// Confirmed node-escape capabilities
	for _, c := range caps {
		if (c.CapabilityID == "ESC_PRIV_POD" || c.CapabilityID == "ESC_HOSTPATH_NODE" || c.CapabilityID == "ESC_RUNTIME_ACTIVE") &&
			(c.State == "confirmed" || c.State == "exploited" || c.State == "chained") {
			score = math.Max(score, 7.0)
			break
		}
	}
	return math.Min(dimMaxBlastRadius, score)
}

// --- helpers ------------------------------------------------------------

func computeTimeDecayV3(insights []models.Insight) float64 {
	if len(insights) == 0 {
		return 0.85 // No insights → slight decay
	}
	var newest time.Time
	for _, ins := range insights {
		if ins.CreatedAt.After(newest) {
			newest = ins.CreatedAt
		}
	}
	age := time.Since(newest)
	if age < 24*time.Hour {
		return 1.0
	}
	if age < 7*24*time.Hour {
		return 0.95
	}
	if age < 30*24*time.Hour {
		return 0.90
	}
	return 0.85
}

func hasClusterAdminAttackPath(paths []models.AttackPath) bool {
	type pathNode struct {
		Type       string                 `json:"type"`
		Properties map[string]interface{} `json:"properties"`
	}
	for _, p := range paths {
		if p.TotalRisk >= 9.0 {
			return true
		}
		if p.Nodes != "" {
			var nodes []pathNode
			if err := json.Unmarshal([]byte(p.Nodes), &nodes); err == nil {
				for _, n := range nodes {
					if !strings.EqualFold(n.Type, "ClusterRole") {
						continue
					}
					name, _ := n.Properties["name"].(string)
					if strings.EqualFold(name, "cluster-admin") {
						return true
					}
				}
			}
		}
		if strings.Contains(strings.ToLower(p.Description), "cluster-admin") {
			return true
		}
	}
	return false
}

// computeDimFreshness maps data age to a 0–1 decay multiplier per the spec's
// freshness table: <5m → 1.0, <1h → 0.8, <24h → 0.5, >24h → 0.2.
func computeDimFreshness(newest time.Time) float64 {
	if newest.IsZero() {
		return 1.0
	}
	age := time.Since(newest)
	switch {
	case age < 5*time.Minute:
		return 1.0
	case age < time.Hour:
		return 0.8
	case age < 24*time.Hour:
		return 0.5
	default:
		return 0.2
	}
}

func newestInsightTime(insights []models.Insight, insightTypes ...string) time.Time {
	var newest time.Time
	for _, ins := range insights {
		for _, t := range insightTypes {
			if strings.EqualFold(ins.InsightType, t) {
				if ins.CreatedAt.After(newest) {
					newest = ins.CreatedAt
				}
				break
			}
		}
	}
	return newest
}

func newestAttackPathTime(paths []models.AttackPath) time.Time {
	var newest time.Time
	for _, p := range paths {
		if p.CreatedAt.After(newest) {
			newest = p.CreatedAt
		}
	}
	return newest
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func newestRuntimeSignalTime(runtimeSignals []models.RuntimeSignal) time.Time {
	var newest time.Time
	for _, rs := range runtimeSignals {
		seenAt := runtimeSignalLastSeenTime(rs)
		if seenAt.After(newest) {
			newest = seenAt
		}
	}
	return newest
}

func parseRFC3339Time(raw *string) time.Time {
	if raw == nil {
		return time.Time{}
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		// Common DB text representations (e.g. Postgres timestamptz scanned into string).
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999Z07",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05Z07",
		// Last-resort: no timezone information (treated as UTC by time.Parse).
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func runtimeSignalLastSeenTime(rs models.RuntimeSignal) time.Time {
	if t := parseRFC3339Time(rs.LastSeenAt); !t.IsZero() {
		return t
	}
	return rs.CreatedAt
}

func runtimeSignalFirstSeenTime(rs models.RuntimeSignal) time.Time {
	if t := parseRFC3339Time(rs.FirstSeenAt); !t.IsZero() {
		return t
	}
	return rs.CreatedAt
}

func runtimePersistenceMinutes(insights []models.Insight, runtimeSignals []models.RuntimeSignal) int {
	var oldest time.Time
	for _, ins := range insights {
		t := strings.ToLower(ins.InsightType)
		if t != "runtime" && t != "runtime_threat" && t != "runtime-behavior" {
			continue
		}
		if oldest.IsZero() || ins.CreatedAt.Before(oldest) {
			oldest = ins.CreatedAt
		}
	}
	for _, rs := range runtimeSignals {
		seenAt := runtimeSignalFirstSeenTime(rs)
		if seenAt.IsZero() {
			continue
		}
		if oldest.IsZero() || seenAt.Before(oldest) {
			oldest = seenAt
		}
	}
	if oldest.IsZero() {
		return 0
	}
	return int(time.Since(oldest).Minutes())
}

func computePathInfluence(paths []models.AttackPath, runtimeSignals []models.RuntimeSignal, signalToSteps map[string][]string, baseRiskAnchor float64, pre *pathRuntimeAlign) pathInfluence {
	pi := pathInfluence{
		ClassCounts: map[string]int{
			"ESCAPE_PATH":     0,
			"LATERAL_PATH":    0,
			"DATA_EXFIL_PATH": 0,
			"PRIV_ESC_PATH":   0,
		},
	}
	if len(paths) == 0 {
		return pi
	}
	var runtimeStepProgress float64
	var matchedSignals []string
	var criticalHitCount int
	if pre != nil {
		runtimeStepProgress = pre.Progress
		matchedSignals = pre.MatchedSignals
		criticalHitCount = pre.CriticalHitCount
	} else {
		stepWeights, stepDepth := collectAttackStepWeightsAndDepth(paths)
		runtimeStepProgress, matchedSignals, criticalHitCount = inferRuntimeStepProgress(stepWeights, runtimeSignals, signalToSteps, stepDepth)
	}
	pi.RuntimeStepProgress = runtimeStepProgress
	pi.MatchedRuntimeSignals = matchedSignals
	pi.CriticalHitCount = criticalHitCount
	pi.ProgressRaw = runtimeStepProgress
	lambda := 2.3
	progress := clamp(runtimeStepProgress, 0, 1)
	pi.ProgressSaturated = math.Round((1-math.Exp(-lambda*progress))*1000) / 1000
	if criticalHitCount >= 2 {
		pi.ProgressSaturated = math.Max(pi.ProgressSaturated, 0.75)
	} else if criticalHitCount == 1 {
		pi.ProgressSaturated = math.Max(pi.ProgressSaturated, 0.6)
	}
	var sumStrength float64
	var sumProgress float64
	seenSignatures := map[string]bool{}
	for _, p := range paths {
		sig := pathSignature(p)
		if seenSignatures[sig] {
			continue
		}
		seenSignatures[sig] = true
		s := clamp(p.TotalRisk/10.0, 0, 1)
		if s > pi.MaxStrength {
			pi.MaxStrength = s
		}
		sumStrength += s
		progress := clamp(float64(countAttackStepNodes(p))/math.Max(float64(p.Length), 1.0), 0, 1)
		sumProgress += progress
		if reachesNodeAsset(p) {
			pi.ReachesNode = true
		}
		for cls := range pi.ClassCounts {
			if strings.HasPrefix(strings.ToUpper(p.Description), cls) {
				pi.ClassCounts[cls]++
				break
			}
		}
	}
	pi.AppliedPathCount = len(seenSignatures)
	if pi.AppliedPathCount > 0 {
		pi.AvgStrength = math.Round((sumStrength/float64(pi.AppliedPathCount))*1000) / 1000
		pi.Progress = math.Round((sumProgress/float64(pi.AppliedPathCount))*1000) / 1000
	}
	pi.RuntimeDecay = runtimeInjectDecay(runtimeSignals)
	pi.GraphBasedE = math.Round((0.25*pi.MaxStrength)*1000) / 1000
	if len(runtimeSignals) == 0 {
		pi.GraphBasedE = math.Round((pi.GraphBasedE*0.8)*1000) / 1000
		if pi.MaxStrength >= 0.6 {
			// Keep latent path signal visible even without runtime confirmation.
			pi.GraphBasedE = math.Max(pi.GraphBasedE, 0.25)
			latentBoost := 0.05 * pi.MaxStrength
			pi.GraphBasedE = math.Round((pi.GraphBasedE+latentBoost)*1000) / 1000
		}
	}
	pi.RuntimeBasedE = math.Round((0.3*pi.ProgressSaturated*pi.RuntimeDecay)*1000) / 1000
	pi.EInjectNorm = math.Round((pi.GraphBasedE+pi.RuntimeBasedE)*1000) / 1000
	pi.RInjectNorm = math.Round((0.20*pi.MaxStrength)*1000) / 1000
	if pi.ReachesNode {
		pi.IInjectNorm = 0.2
	}
	if criticalHitCount > 0 {
		criticalImpact := 0.08 + 0.06*math.Min(1.0, float64(criticalHitCount)/2.0)
		pi.IInjectNorm += criticalImpact
	}
	impactBaseScale := 0.55 + 0.45*clamp(baseRiskAnchor, 0, 1)
	pi.IInjectNorm *= impactBaseScale
	effectiveStrength := math.Min(pi.MaxStrength, 0.9)
	pi.InjectScale = math.Round((0.5+0.5*effectiveStrength)*1000) / 1000
	pi.EInjectNorm *= pi.InjectScale
	pi.RInjectNorm *= pi.InjectScale
	pi.IInjectNorm *= pi.InjectScale
	uniqueClasses := 0
	for _, c := range pi.ClassCounts {
		if c > 0 {
			uniqueClasses++
		}
	}
	pi.DiversityFactor = 1.0
	if pi.AppliedPathCount >= 2 {
		ratio := float64(uniqueClasses) / float64(pi.AppliedPathCount)
		ratio = clamp(ratio, 0, 1)
		pi.DiversityFactor = math.Round((0.7+0.3*ratio)*1000) / 1000
		pi.EInjectNorm *= pi.DiversityFactor
		pi.RInjectNorm *= pi.DiversityFactor
		pi.IInjectNorm *= pi.DiversityFactor
	}
	// Guardrail: low-base entities should not get path-dominant injections.
	if baseRiskAnchor < 0.3 {
		pi.EInjectNorm *= 0.7
		pi.RInjectNorm *= 0.7
		pi.IInjectNorm *= 0.7
	}
	// Guardrail: keep global cap, but reserve graph-based latent channel.
	// Runtime-driven and impact/reach channels are capped first; graph channel is
	// kept available to preserve strong-path latent risk semantics.
	totalInjectNorm := pi.EInjectNorm + pi.RInjectNorm + pi.IInjectNorm
	graphReserved := math.Min(pi.GraphBasedE*pi.InjectScale*pi.DiversityFactor, 0.2)
	if totalInjectNorm > 0.4 {
		nonGraphE := math.Max(0, pi.EInjectNorm-graphReserved)
		adjustable := nonGraphE + pi.RInjectNorm + pi.IInjectNorm
		if adjustable > 0 {
			scale := (0.4 - graphReserved) / adjustable
			scale = clamp(scale, 0, 1)
			pi.EInjectNorm = graphReserved + nonGraphE*scale
			pi.RInjectNorm *= scale
			pi.IInjectNorm *= scale
		}
	}
	pi.EInjectNorm = math.Round(pi.EInjectNorm*1000) / 1000
	pi.RInjectNorm = math.Round(pi.RInjectNorm*1000) / 1000
	pi.IInjectNorm = math.Round(pi.IInjectNorm*1000) / 1000
	pi.EInjectPoints = math.Round((pi.EInjectNorm*dimMaxAttackPath)*100) / 100
	pi.RInjectPoints = math.Round((pi.RInjectNorm*dimMaxExposure)*100) / 100
	pi.IInjectPoints = math.Round((pi.IInjectNorm*dimMaxBlastRadius)*100) / 100
	return pi
}

func collectAttackStepWeightsAndDepth(paths []models.AttackPath) (map[string]float64, map[string]int) {
	out := map[string]float64{}
	minIdx := map[string]int{}
	for _, p := range paths {
		if p.Nodes == "" {
			continue
		}
		type nodeShape struct {
			ID         string                 `json:"id"`
			Type       string                 `json:"type"`
			Properties map[string]interface{} `json:"properties"`
		}
		var nodes []nodeShape
		if err := json.Unmarshal([]byte(p.Nodes), &nodes); err != nil {
			continue
		}
		totalNodes := len(nodes)
		for idx, n := range nodes {
			if !strings.EqualFold(n.Type, "attack_step") && !strings.EqualFold(n.Type, "AttackStep") {
				continue
			}
			posNorm := clamp(float64(idx)/math.Max(float64(totalNodes-1), 1), 0, 1)
			weight := 1.0
			if posNorm >= 0.66 {
				weight = 1.5
			} else if posNorm >= 0.33 {
				weight = 1.2
			}
			stepIDCandidate := ""
			if v, ok := n.Properties["stepId"].(string); ok && v != "" {
				stepIDCandidate = v
			} else if strings.HasPrefix(n.ID, "step:") {
				parts := strings.Split(n.ID, ":")
				if len(parts) >= 3 {
					stepIDCandidate = parts[len(parts)-1]
				}
			}
			category := ""
			if v, ok := n.Properties["category"].(string); ok {
				category = v
			}
			weight *= semanticStepWeight(stepIDCandidate, category)
			assign := func(stepID string) {
				stepID = strings.ToUpper(strings.TrimSpace(stepID))
				if stepID == "" {
					return
				}
				if prev, ok := out[stepID]; !ok || weight > prev {
					out[stepID] = weight
					minIdx[stepID] = idx
				} else if ok && weight == prev {
					if prevD, okd := minIdx[stepID]; !okd || idx < prevD {
						minIdx[stepID] = idx
					}
				}
			}
			if v, ok := n.Properties["stepId"].(string); ok && v != "" {
				assign(v)
				continue
			}
			if strings.HasPrefix(n.ID, "step:") {
				parts := strings.Split(n.ID, ":")
				if len(parts) >= 3 {
					assign(parts[len(parts)-1])
				}
			}
		}
	}
	return out, minIdx
}

func semanticStepWeight(stepID, category string) float64 {
	sid := strings.ToUpper(strings.TrimSpace(stepID))
	cat := strings.ToUpper(strings.TrimSpace(category))
	switch {
	case strings.Contains(cat, "CREDENTIAL"), strings.Contains(sid, "CRED"), strings.Contains(sid, "TOKEN"):
		return 1.5
	case strings.Contains(cat, "PRIV"), strings.Contains(sid, "ESC"), strings.Contains(sid, "ROOT"):
		return 1.4
	case strings.Contains(cat, "EXEC"), strings.Contains(sid, "EXEC"):
		return 1.2
	default:
		return 1.0
	}
}

func pathSignature(p models.AttackPath) string {
	edges := strings.TrimSpace(p.Edges)
	if edges != "" {
		return "e:" + edges
	}
	return strings.TrimSpace(p.Nodes) + "|" + strings.TrimSpace(p.Description)
}

func inferRuntimeStepProgress(stepWeights map[string]float64, runtimeSignals []models.RuntimeSignal, signalToSteps map[string][]string, stepDepth map[string]int) (float64, []string, int) {
	if len(stepWeights) == 0 || len(runtimeSignals) == 0 {
		return 0, nil, 0
	}
	matchedSteps := map[string]bool{}
	matchedSignals := map[string]bool{}
	stepMatchConf := map[string]float64{}
	for _, rs := range runtimeSignals {
		st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
		conf := rs.Confidence
		if conf <= 0 {
			conf = 0.5
		}
		if conf > 1 {
			conf = 1
		}
		cands := signalToSteps[st]
		for _, sid := range cands {
			stepID := strings.ToUpper(sid)
			if _, ok := stepWeights[stepID]; ok {
				matchedSteps[stepID] = true
				matchedSignals[st] = true
				if conf > stepMatchConf[stepID] {
					stepMatchConf[stepID] = conf
				}
			}
		}
	}
	if len(stepWeights) == 0 {
		return 0, nil, 0
	}
	totalWeight := 0.0
	matchedWeight := 0.0
	for sid, w := range stepWeights {
		totalWeight += w
		if matchedSteps[sid] {
			c := stepMatchConf[sid]
			depth := 0
			if stepDepth != nil {
				depth = stepDepth[sid]
			}
			depthPenalty := math.Max(0.7, 1.0-0.1*float64(depth))
			matchedWeight += w * (1.0 + 0.3*c) * depthPenalty
		}
	}
	progress := 0.0
	if totalWeight > 0 {
		progress = clamp(matchedWeight/totalWeight, 0, 1)
	}
	outSignals := make([]string, 0, len(matchedSignals))
	for s := range matchedSignals {
		outSignals = append(outSignals, s)
	}
	sort.Strings(outSignals)
	criticalHits := 0
	for sid := range matchedSteps {
		if isCriticalAttackStep(sid) {
			criticalHits++
		}
	}
	return math.Round(progress*1000) / 1000, outSignals, criticalHits
}

func isCriticalAttackStep(stepID string) bool {
	sid := strings.ToUpper(strings.TrimSpace(stepID))
	switch sid {
	case "CREDENTIAL_ACCESS", "NODE_CRED_DUMP", "PRIV_ESC", "HOST_ESCAPE":
		return true
	default:
		return false
	}
}

func runtimeInjectDecay(runtimeSignals []models.RuntimeSignal) float64 {
	if len(runtimeSignals) == 0 {
		return 0.35
	}
	newest := time.Time{}
	for _, rs := range runtimeSignals {
		seenAt := runtimeSignalLastSeenTime(rs)
		if seenAt.After(newest) {
			newest = seenAt
		}
	}
	if newest.IsZero() {
		return 0
	}
	ageHours := math.Max(0, time.Since(newest).Hours())
	const hardCutHours = 24.0
	if ageHours >= hardCutHours {
		return 0
	}
	decay := math.Exp(-ageHours / 12.0)
	return clamp(decay, 0, 1.0)
}

type mappingVersionMeta struct {
	MappingHash          string `json:"mapping_hash"`
	MappingCount         int    `json:"mapping_count"`
	MappingEffectiveTime string `json:"mapping_effective_time"`
	MappingVersionID     string `json:"mapping_version_id"`
}

var runtimeSignalToStepsFallback = map[string][]string{
	"PROC_ROOT_PIVOT":               {"PROC_ROOT_PIVOT"},
	"NAMESPACE_ESCAPE":              {"PROC_NAMESPACE_ACCESS", "IPC_NAMESPACE_ACCESS"},
	"FS_ESCAPE_ATTEMPT":             {"NODE_FS_WRITE"},
	"CAPABILITY_MISUSE":             {"NODE_KERNEL_ACCESS", "NETWORK_SNIFFING"},
	"EBPF_EXEC_ACTIVITY":            {"NODE_PERSISTENCE"},
	"SUSPICIOUS_EXEC_FROM_SNAPSHOT": {"NODE_PERSISTENCE"},
	"NETWORK_QUEUE_ANOMALY":         {"NETWORK_SNIFFING"},
}

func loadRuntimeSignalStepMappings(db *gorm.DB) map[string][]string {
	m, _ := loadRuntimeSignalStepMappingsWithMeta(db)
	return m
}

func loadRuntimeSignalStepMappingsWithMeta(db *gorm.DB) (map[string][]string, mappingVersionMeta) {
	if db == nil || !db.Migrator().HasTable(&models.RuntimeSignalStepMapping{}) {
		return runtimeSignalToStepsFallback, mappingVersionMeta{}
	}
	var rows []models.RuntimeSignalStepMapping
	now := time.Now()
	if err := db.Where("enabled = ? AND (effective_from IS NULL OR effective_from <= ?)", true, now).Find(&rows).Error; err != nil || len(rows) == 0 {
		return runtimeSignalToStepsFallback, mappingVersionMeta{}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SignalType == rows[j].SignalType {
			return rows[i].StepID < rows[j].StepID
		}
		return rows[i].SignalType < rows[j].SignalType
	})
	out := map[string][]string{}
	const maxStepsPerSignal = 2
	latest := time.Time{}
	for _, r := range rows {
		st := strings.ToUpper(strings.TrimSpace(r.SignalType))
		sid := strings.ToUpper(strings.TrimSpace(r.StepID))
		if st == "" || sid == "" {
			continue
		}
		if len(out[st]) >= maxStepsPerSignal {
			continue
		}
		out[st] = append(out[st], sid)
		if r.EffectiveFrom != nil && r.EffectiveFrom.After(latest) {
			latest = *r.EffectiveFrom
		}
	}
	if len(out) == 0 {
		return runtimeSignalToStepsFallback, mappingVersionMeta{}
	}
	type mappingShape struct {
		SignalType    string     `json:"signal_type"`
		StepID        string     `json:"step_id"`
		EffectiveFrom *time.Time `json:"effective_from,omitempty"`
	}
	shapes := make([]mappingShape, 0, len(rows))
	for _, r := range rows {
		shapes = append(shapes, mappingShape{
			SignalType:    strings.ToUpper(strings.TrimSpace(r.SignalType)),
			StepID:        strings.ToUpper(strings.TrimSpace(r.StepID)),
			EffectiveFrom: r.EffectiveFrom,
		})
	}
	payload, _ := json.Marshal(shapes)
	h := sha256.Sum256(payload)
	meta := mappingVersionMeta{
		MappingHash:  hex.EncodeToString(h[:]),
		MappingCount: len(rows),
	}
	if len(meta.MappingHash) >= 16 {
		meta.MappingVersionID = "mv-" + meta.MappingHash[:16]
	}
	if !latest.IsZero() {
		meta.MappingEffectiveTime = latest.UTC().Format(time.RFC3339)
	}
	return out, meta
}

func countAttackStepNodes(path models.AttackPath) int {
	if path.Nodes == "" {
		return 0
	}
	type nodeShape struct {
		Type string `json:"type"`
	}
	var nodes []nodeShape
	if err := json.Unmarshal([]byte(path.Nodes), &nodes); err != nil {
		return 0
	}
	cnt := 0
	for _, n := range nodes {
		if strings.EqualFold(n.Type, "attack_step") || strings.EqualFold(n.Type, "AttackStep") {
			cnt++
		}
	}
	return cnt
}

func reachesNodeAsset(path models.AttackPath) bool {
	if path.Nodes == "" {
		return false
	}
	type nodeShape struct {
		Type string `json:"type"`
	}
	var nodes []nodeShape
	if err := json.Unmarshal([]byte(path.Nodes), &nodes); err != nil {
		return false
	}
	for _, n := range nodes {
		if strings.EqualFold(n.Type, "node") || strings.EqualFold(n.Type, "Node") {
			return true
		}
	}
	return false
}
