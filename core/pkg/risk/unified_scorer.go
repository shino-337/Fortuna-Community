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
// Backward compatibility: V2 scorer continues to run unchanged.  V3 writes its
// own fields (scorer_version="v3") so both records co-exist until V2 is retired.
package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fortuna/core/pkg/models"
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

// Toxic combo boosts (additive, applied after dimension sum)
const (
	toxicBoostCVEInternetExposed    = 10.0
	toxicBoostPrivEscClusterAdmin   = 15.0
	toxicBoostTokenTheftEgress      = 10.0
)

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

// CalculateScoreV3 computes the unified risk score for a resource identified by
// its UID.  The function is safe to call concurrently for different resources.
func (s *UnifiedScorerV3) CalculateScoreV3(ctx context.Context, resourceUID string) (*UnifiedScoreV3, error) {
	if resourceUID == "" {
		return nil, fmt.Errorf("resourceUID must not be empty")
	}

	// --- 1. Load resource info ---
	var pod models.Pod
	podFound := false
	if err := s.db.WithContext(ctx).
		Where("uid = ? AND deleted_at IS NULL", resourceUID).
		First(&pod).Error; err == nil {
		podFound = true
	}

	resourceName, resourceType, namespace, clusterID := resourceUID, "pod", "", ""
	if podFound {
		resourceName = pod.Name
		namespace = pod.Namespace
		clusterID = pod.ClusterID
	}

	// --- 2. Load insights ---
	var insights []models.Insight
	if err := s.db.WithContext(ctx).
		Where("resource_uid = ? AND status = 'active' AND deleted_at IS NULL", resourceUID).
		Find(&insights).Error; err != nil {
		log.Printf("[UnifiedScorerV3] failed to load insights for %s: %v", resourceUID, err)
	}

	// --- 3. Load pod capabilities ---
	var caps []models.PodCapability
	if s.db.Migrator().HasTable("pod_capabilities") {
		if err := s.db.WithContext(ctx).
			Where("pod_uid = ?", resourceUID).
			Find(&caps).Error; err != nil {
			log.Printf("[UnifiedScorerV3] failed to load capabilities for %s: %v", resourceUID, err)
		}
	}

	// --- 4. Load persisted attack paths ---
	var attackPaths []models.AttackPath
	if s.db.Migrator().HasTable("attack_paths") {
		if err := s.db.WithContext(ctx).
			Where("pod_uid = ?", resourceUID).
			Find(&attackPaths).Error; err != nil {
			log.Printf("[UnifiedScorerV3] failed to load attack paths for %s: %v", resourceUID, err)
		}
	}

	// --- 5. Load runtime signals (24h window) ---
	var runtimeSignalCount int64
	if s.db.Migrator().HasTable("runtime_signals") {
		s.db.WithContext(ctx).Model(&models.RuntimeSignal{}).
			Where("pod_uid = ? AND created_at > ?", resourceUID, time.Now().Add(-24*time.Hour)).
			Count(&runtimeSignalCount)
	}

	// --- 6. Compute dimensions ---
	vulnScore := s.scoreVulnerability(insights)
	capScore := s.scoreCapabilityExposure(caps)
	pathScore := s.scoreAttackPathDim(attackPaths)
	rbacScore := s.scoreRBACPolicy(insights)
	runtimeScore := s.scoreRuntimeThreat(insights, int(runtimeSignalCount))
	exposureScore := s.scoreExposureDim(pod, podFound)
	blastScore := s.scoreBlastRadiusDim(attackPaths, caps)

	dimSum := vulnScore + capScore + pathScore + rbacScore + runtimeScore + exposureScore + blastScore

	// --- 7. Toxic combo boosts ---
	var toxicCombos []string
	toxicBoost := 0.0

	isCriticalCVE := hasCriticalCVEInsight(insights)
	isInternetExposed := podFound && (pod.HostNetwork)
	hasPrivileged := hasCapabilityID(caps, "ESC_PRIV_POD")
	hasEscapeConfirmed := hasCapabilityStateAtLeast(caps, "ESC_PRIV_POD", "confirmed") ||
		hasCapabilityStateAtLeast(caps, "ESC_HOSTPATH_NODE", "confirmed") ||
		hasCapabilityStateAtLeast(caps, "ESC_RUNTIME_ACTIVE", "confirmed")
	hasClusterAdminPath := hasClusterAdminAttackPath(attackPaths)
	hasTokenTheft := hasCapabilityID(caps, "ID_TOKEN_POD")
	hasExternalEgress := hasRuntimeInsight(insights, "external_egress")

	if isCriticalCVE && isInternetExposed {
		toxicCombos = append(toxicCombos, "cve_critical+internet_exposed")
		toxicBoost += toxicBoostCVEInternetExposed
	}
	if hasPrivileged && hasEscapeConfirmed && hasClusterAdminPath {
		toxicCombos = append(toxicCombos, "privileged+escape_confirmed+cluster_admin_path")
		toxicBoost += toxicBoostPrivEscClusterAdmin
	}
	if hasTokenTheft && hasExternalEgress {
		toxicCombos = append(toxicCombos, "token_theft+external_egress")
		toxicBoost += toxicBoostTokenTheftEgress
	}

	// --- 8. Time decay ---
	timeDecay := computeTimeDecayV3(insights)

	// --- 9. Final score ---
	rawScore := (dimSum + toxicBoost) * timeDecay
	totalScore := math.Min(100.0, math.Max(0.0, rawScore))
	totalScore = math.Round(totalScore*100) / 100

	priorityLevel := models.GetPriorityLevelV2(totalScore)

	factors := map[string]interface{}{
		"scorer_version": "v3",
		"dimensions": map[string]interface{}{
			"vulnerability":       math.Round(vulnScore*100) / 100,
			"capability_exposure": math.Round(capScore*100) / 100,
			"attack_path":         math.Round(pathScore*100) / 100,
			"rbac_policy":         math.Round(rbacScore*100) / 100,
			"runtime_threat":      math.Round(runtimeScore*100) / 100,
			"exposure":            math.Round(exposureScore*100) / 100,
			"blast_radius":        math.Round(blastScore*100) / 100,
		},
		"toxic_combos":        toxicCombos,
		"time_decay":          math.Round(timeDecay*100) / 100,
		"runtime_signals_24h": runtimeSignalCount,
		"insights_count":      len(insights),
		"capabilities_count":  len(caps),
		"attack_paths_count":  len(attackPaths),
	}

	return &UnifiedScoreV3{
		ResourceUID:             resourceUID,
		ResourceName:            resourceName,
		ResourceType:            resourceType,
		Namespace:               namespace,
		ClusterID:               clusterID,
		PriorityLevel:           priorityLevel,
		TotalScore:              totalScore,
		TimeDecay:               math.Round(timeDecay*100) / 100,
		ScorerVersion:           "v3",
		VulnerabilityScore:      math.Round(vulnScore*100) / 100,
		CapabilityExposureScore: math.Round(capScore*100) / 100,
		AttackPathScore:         math.Round(pathScore*100) / 100,
		RBACPolicyScore:         math.Round(rbacScore*100) / 100,
		RuntimeThreatScore:      math.Round(runtimeScore*100) / 100,
		ExposureScore:           math.Round(exposureScore*100) / 100,
		BlastRadiusScore:        math.Round(blastScore*100) / 100,
		ToxicCombos:             toxicCombos,
		Factors:                 factors,
		CalculatedAt:            time.Now().UTC(),
	}, nil
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
			Columns: []clause.Column{{Name: "resource_uid"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"total_score", "base_score", "exploitability_score", "business_impact_score",
				"time_decay", "scorer_version", "factors", "priority_level", "calculated_at",
				"capability_exposure_score", "attack_path_score", "runtime_threat_score", "blast_radius_score",
				"resource_name", "namespace", "cluster_id", "updated_at",
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
		log.Printf("[UnifiedScorerV3] score updated for %s total=%.1f priority=%s", resourceUID, score.TotalScore, score.PriorityLevel)
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
func (s *UnifiedScorerV3) scoreAttackPathDim(paths []models.AttackPath) float64 {
	if len(paths) == 0 {
		return 0
	}

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

	return math.Min(dimMaxAttackPath, base+criticalBonus)
}

// scoreRBACPolicy scores up to dimMaxRBACPolicy based on RBAC/pod-security insights.
func (s *UnifiedScorerV3) scoreRBACPolicy(insights []models.Insight) float64 {
	var score float64
	for _, ins := range insights {
		t := strings.ToLower(ins.InsightType)
		if t != "rbac" && t != "pod-security" && t != "capability" {
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

// scoreRuntimeThreat scores up to dimMaxRuntimeThreat.
func (s *UnifiedScorerV3) scoreRuntimeThreat(insights []models.Insight, signalCount24h int) float64 {
	var insightScore float64
	for _, ins := range insights {
		t := strings.ToLower(ins.InsightType)
		if t != "runtime" && t != "runtime_threat" {
			continue
		}
		switch strings.ToLower(ins.Severity) {
		case "critical":
			insightScore = math.Max(insightScore, 12.0)
		case "high":
			insightScore = math.Max(insightScore, 9.0)
		case "medium":
			insightScore = math.Max(insightScore, 5.0)
		}
	}

	// Signal velocity bonus
	signalBonus := math.Min(3.0, math.Log1p(float64(signalCount24h))*1.0)
	return math.Min(dimMaxRuntimeThreat, insightScore+signalBonus)
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

func hasCriticalCVEInsight(insights []models.Insight) bool {
	for _, ins := range insights {
		if strings.EqualFold(ins.InsightType, "vulnerability") &&
			strings.EqualFold(ins.Severity, "critical") {
			return true
		}
	}
	return false
}

func hasCapabilityID(caps []models.PodCapability, id string) bool {
	for _, c := range caps {
		if c.CapabilityID == id {
			return true
		}
	}
	return false
}

func hasCapabilityStateAtLeast(caps []models.PodCapability, id, minState string) bool {
	stateOrder := map[string]int{
		"detected":  0,
		"confirmed": 1,
		"exploited": 2,
		"chained":   3,
	}
	minOrder, ok := stateOrder[minState]
	if !ok {
		return false
	}
	for _, c := range caps {
		if c.CapabilityID == id {
			if ord, ok2 := stateOrder[c.State]; ok2 && ord >= minOrder {
				return true
			}
		}
	}
	return false
}

func hasClusterAdminAttackPath(paths []models.AttackPath) bool {
	for _, p := range paths {
		if strings.Contains(strings.ToLower(p.Description), "cluster-admin") ||
			strings.Contains(strings.ToLower(p.Description), "full cluster access") {
			return true
		}
	}
	return false
}

func hasRuntimeInsight(insights []models.Insight, keyword string) bool {
	for _, ins := range insights {
		if strings.Contains(strings.ToLower(ins.Title), keyword) ||
			strings.Contains(strings.ToLower(ins.InsightType), keyword) {
			return true
		}
	}
	return false
}
