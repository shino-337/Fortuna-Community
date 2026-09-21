package risk

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/k8scorroboration"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"gorm.io/gorm"
)

// CalculateScoreV3ForIdentity computes V3 using only evidence owned by one
// canonical {cluster_id,pod_uid} identity. It is the production multi-cluster
// scorer; the UID-only API remains a transition surface until all callers move.
func (s *UnifiedScorerV3) CalculateScoreV3ForIdentity(ctx context.Context, id resourceidentity.Identity) (*UnifiedScoreV3, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("unified scorer database is nil")
	}
	if err := id.Validate(); err != nil {
		return nil, err
	}

	var pod models.Pod
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", id.ClusterID, id.ResourceUID).
		First(&pod).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("pod identity not found: %s/%s", id.ClusterID, id.ResourceUID)
		}
		return nil, err
	}
	resourceName, resourceType, namespace := pod.Name, "pod", pod.Namespace

	var insights []models.Insight
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ? AND resource_uid = ? AND status IN ('active', 'acknowledged') AND deleted_at IS NULL", id.ClusterID, id.ResourceUID).
		Find(&insights).Error; err != nil {
		return nil, fmt.Errorf("load scoped insights: %w", err)
	}

	var caps []models.PodCapability
	if s.db.Migrator().HasTable("pod_capabilities") {
		if err := s.db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, id.ResourceUID).
			Find(&caps).Error; err != nil {
			return nil, fmt.Errorf("load scoped capabilities: %w", err)
		}
	}

	var attackPaths []models.AttackPath
	if s.db.Migrator().HasTable("attack_paths") {
		if err := s.db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, id.ResourceUID).
			Find(&attackPaths).Error; err != nil {
			return nil, fmt.Errorf("load scoped attack paths: %w", err)
		}
	}

	var runtimeSignalEvents24h int64
	var runtimeBurstEvents5m int64
	var runtimeUniqueSignalTypes5m int64
	var runtimeSignals []models.RuntimeSignal
	if s.db.Migrator().HasTable("runtime_signals") {
		since24h := time.Now().Add(-24 * time.Hour)
		since5m := time.Now().Add(-5 * time.Minute)
		var sum24 struct {
			Total int64 `gorm:"column:total"`
		}
		if err := s.db.WithContext(ctx).Model(&models.RuntimeSignal{}).
			Select("COALESCE(SUM(count),0) as total").
			Where("cluster_id = ? AND pod_uid = ? AND (created_at > ? OR last_seen_at > ?)", id.ClusterID, id.ResourceUID, since24h, since24h).
			Scan(&sum24).Error; err != nil {
			return nil, fmt.Errorf("sum scoped runtime signals: %w", err)
		}
		runtimeSignalEvents24h = sum24.Total
		if err := s.db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ? AND (created_at > ? OR last_seen_at > ?)", id.ClusterID, id.ResourceUID, since24h, since24h).
			Find(&runtimeSignals).Error; err != nil {
			return nil, fmt.Errorf("load scoped runtime signals: %w", err)
		}
		if err := s.db.WithContext(ctx).Model(&models.RuntimeSignal{}).
			Distinct("signal_type").
			Where("cluster_id = ? AND pod_uid = ? AND (created_at > ? OR last_seen_at > ?)", id.ClusterID, id.ResourceUID, since5m, since5m).
			Count(&runtimeUniqueSignalTypes5m).Error; err != nil {
			return nil, fmt.Errorf("count scoped runtime signal types: %w", err)
		}
	}

	var runtimeEvents24h []models.RuntimeEvent
	if s.db.Migrator().HasTable("runtime_events") {
		since24h := time.Now().Add(-24 * time.Hour)
		if err := s.db.WithContext(ctx).
			Where("cluster_id = ? AND pod_uid = ? AND created_at > ?", id.ClusterID, id.ResourceUID, since24h).
			Find(&runtimeEvents24h).Error; err != nil {
			return nil, fmt.Errorf("load scoped runtime events: %w", err)
		}
		runtimeEvents24h = filterRuntimeEventsScoringConfidence(runtimeEvents24h)
		since5m := time.Now().Add(-5 * time.Minute)
		var observed, created int64
		if err := s.db.WithContext(ctx).Model(&models.RuntimeEvent{}).
			Where("cluster_id = ? AND pod_uid = ? AND observed_at IS NOT NULL AND observed_at > ? AND COALESCE(confidence, 0) > 0", id.ClusterID, id.ResourceUID, since5m).
			Count(&observed).Error; err != nil {
			return nil, err
		}
		if err := s.db.WithContext(ctx).Model(&models.RuntimeEvent{}).
			Where("cluster_id = ? AND pod_uid = ? AND observed_at IS NULL AND created_at > ? AND COALESCE(confidence, 0) > 0", id.ClusterID, id.ResourceUID, since5m).
			Count(&created).Error; err != nil {
			return nil, err
		}
		runtimeBurstEvents5m = observed + created
	}

	caps, runtimeDerivedCaps := mergeRuntimeDerivedPodCapabilities(id.ResourceUID, namespace, caps, runtimeSignals, runtimeEvents24h)
	signalToSteps, mappingMeta := loadRuntimeSignalStepMappingsWithMeta(s.db)
	stepWeights, stepDepthByID := collectAttackStepWeightsAndDepth(attackPaths)
	runtimeAlignProgress, matchedRuntimeForPath, runtimeCritHits := inferRuntimeStepProgress(stepWeights, runtimeSignals, signalToSteps, stepDepthByID)

	vulnScore := s.scoreVulnerability(insights)
	capScore := s.scoreCapabilityExposure(caps)
	pathDim := s.scoreAttackPathDim(attackPaths)
	capIDsForCK := graph.FilterPodCapabilitiesForCKDB(attackPaths, caps)
	ckdbCapabilityRisk := graph.SumCKDBRiskForCapabilityIDs(capIDsForCK)
	if len(runtimeDerivedCaps) > 0 {
		ckdbCapabilityRisk *= 0.85
	}
	rbacScore := s.scoreRBACPolicy(insights)
	temporalCoherence := ComputeRuntimeTemporalCoherence(runtimeEvents24h, runtimeSignals)
	conflictMul := RuntimeConflictMultiplier(runtimeSignals, runtimeEvents24h)
	runtimeScore, runtimeThreatMeta := computeRuntimeThreatWithMeta(insights, runtimeSignalEvents24h, runtimeSignals, runtimeAlignProgress, runtimeEvents24h, temporalCoherence, conflictMul)
	exposureScore := s.scoreExposureDim(pod, true)
	blastScore := s.scoreBlastRadiusDim(attackPaths, caps)
	baseRiskAnchor := clamp((vulnScore+capScore+rbacScore+runtimeScore)/60.0, 0, 1)
	pathInf := computePathInfluence(attackPaths, runtimeSignals, signalToSteps, baseRiskAnchor, &pathRuntimeAlign{
		Progress:         runtimeAlignProgress,
		MatchedSignals:   matchedRuntimeForPath,
		CriticalHitCount: runtimeCritHits,
	})
	pathCore := pathDim + ckdbCapabilityRisk + pathInf.EInjectPoints
	runtimeInfMul := 1.0 + runtimePathInfluenceMultiplier(runtimeScore, runtimeAlignProgress, temporalCoherence.CoherenceMultiplier)
	dimSumEst := vulnScore + capScore + rbacScore + runtimeScore + exposureScore + blastScore + pathDim + ckdbCapabilityRisk + pathInf.EInjectPoints
	if dimSumEst > 1e-6 && runtimeScore/dimSumEst > 0.6 && runtimeInfMul > 1.11 {
		runtimeThreatMeta["runtime_dominance_influence_clamp"] = true
		runtimeThreatMeta["runtime_dominance_ratio_estimate"] = math.Round((runtimeScore/dimSumEst)*1000) / 1000
		runtimeInfMul = 1.11
	}
	pathCore *= runtimeInfMul
	runtimeThreatMeta["runtime_attack_path_influence_mul"] = math.Round(runtimeInfMul*1000) / 1000
	runtimeInfluence := math.Max(0, runtimeInfMul-1.0)
	capMul := 1.0 + runtimeInfluence*0.5
	blastMul := 1.0 + runtimeInfluence*0.3
	capScore = math.Min(dimMaxCapabilityExposure, capScore*capMul)
	blastScore = math.Min(dimMaxBlastRadius, blastScore*blastMul)
	runtimeThreatMeta["runtime_influence_capability_mul"] = math.Round(capMul*1000) / 1000
	runtimeThreatMeta["runtime_influence_blast_mul"] = math.Round(blastMul*1000) / 1000

	pathMitreSet := graph.MitreIDSetFromModels(attackPaths)
	mitrePathBoost, scorerCorrPrec, mitreBoostMeta := graph.ComputeAttackPathMitreBoostForIdentity(s.db, id, 7*24*time.Hour, pathMitreSet)
	pathScore := math.Min(dimMaxAttackPath, pathCore+mitrePathBoost)
	exposureScore = math.Min(dimMaxExposure, exposureScore+pathInf.RInjectPoints)
	blastScore = math.Min(dimMaxBlastRadius, blastScore+pathInf.IInjectPoints)

	ctxInfo := ResolveRiskContextMultiplier(namespace, systemNamespaces[pod.Namespace], pod.HostNetwork, exposureScore)
	vulnFreshness := computeDimFreshness(newestInsightTime(insights, "vulnerability"))
	rbacFreshness := computeDimFreshness(newestInsightTime(insights, "rbac", "pod-security"))
	runtimeFreshness := computeDimFreshness(maxTime(
		newestInsightTime(insights, "runtime", "runtime_threat", "runtime-behavior"),
		newestRuntimeSignalTime(runtimeSignals),
	))
	pathFreshness := computeDimFreshness(newestAttackPathTime(attackPaths))
	timeDecay := computeTimeDecayV3(insights)

	identityKey, err := id.Key()
	if err != nil {
		return nil, err
	}
	aggEngine := NewRiskAggregationEngineV3(
		map[string]float64{
			"vulnerability": 15,
			"capability":    15,
			"attack_path":   15,
			"rbac_policy":   15,
			"runtime":       15,
			"exposure":      15,
			"blast_radius":  10,
		},
		DefaultDimensionWeights,
		DefaultSourceConfidence,
		ctxInfo.EffectiveMultiplier,
	)
	agg := aggEngine.ComputeScore([]RiskFactor{
		{FactorID: "DIM_VULNERABILITY", Category: "vulnerability", Scope: "pod", Source: "insight", DedupeKey: "dim:vulnerability:" + identityKey, Weight: vulnScore, Freshness: vulnFreshness},
		{FactorID: "DIM_CAPABILITY_EXPOSURE", Category: "capability", Scope: "pod", Source: "capability", DedupeKey: "dim:capability:" + identityKey, Weight: capScore},
		{FactorID: "DIM_ATTACK_PATH", Category: "attack_path", Scope: "pod", Source: "attack_path", DedupeKey: "dim:attack_path:" + identityKey, Weight: pathScore, Freshness: pathFreshness},
		{FactorID: "DIM_RBAC_POLICY", Category: "rbac_policy", Scope: "pod", Source: "policy", DedupeKey: "dim:rbac_policy:" + identityKey, Weight: rbacScore, Freshness: rbacFreshness},
		{FactorID: "DIM_RUNTIME_THREAT", Category: "runtime", Scope: "pod", Source: "runtime", DedupeKey: "dim:runtime:" + identityKey, Weight: runtimeScore, Freshness: runtimeFreshness},
		{FactorID: "DIM_EXPOSURE", Category: "exposure", Scope: "pod", Source: "pod", DedupeKey: "dim:exposure:" + identityKey, Weight: exposureScore},
		{FactorID: "DIM_BLAST_RADIUS", Category: "blast_radius", Scope: "pod", Source: "attack_path", DedupeKey: "dim:blast_radius:" + identityKey, Weight: blastScore, Freshness: pathFreshness},
	})

	toxicCombos := make([]string, 0, len(agg.InteractionCombos))
	for _, combo := range agg.InteractionCombos {
		toxicCombos = append(toxicCombos, combo.Name)
	}

	rawScore := agg.TotalScoreRaw * timeDecay
	var previousScore *float64
	var prev models.RiskScore
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ? AND resource_uid = ? AND resource_type = ? AND scorer_version = ? AND deleted_at IS NULL", id.ClusterID, id.ResourceUID, resourceType, "v3").
		Order("calculated_at DESC").
		First(&prev).Error; err == nil {
		value := prev.TotalScore
		previousScore = &value
	}
	trendDelta := 0.0
	if previousScore != nil {
		trendDelta = rawScore - *previousScore
	}
	persistenceMinutes := runtimePersistenceMinutes(insights, runtimeSignals)
	totalScore, temporal := ComputeTemporalScore(rawScore, previousScore, TemporalSignals{
		BurstEvents5m:       int(runtimeBurstEvents5m),
		UniqueSignalTypes5m: int(runtimeUniqueSignalTypes5m),
		TrendDelta:          trendDelta,
		PersistenceMinutes:  persistenceMinutes,
	})

	corroborationK8s := EvaluateK8sAPICorroboration(runtimeEvents24h, runtimeSignals)
	saTokSatW := k8scorroboration.SATokenSatisfactionWeight(runtimeEvents24h, runtimeSignals, time.Now().UTC(), k8scorroboration.DefaultSATokenSatisfactionHalfLife)
	capValConf := meanRuntimeDerivedCapabilityConfidence(caps)
	runtimeConfSc := meanWeightedRuntimeSignalConfidence(runtimeSignals)
	factors := map[string]interface{}{
		"scorer_version":                     "v3",
		"ckdb_capability_risk_points":        math.Round(ckdbCapabilityRisk*100) / 100,
		"mitre_runtime_attack_path_boost":    math.Round(mitrePathBoost*100) / 100,
		"mitre_correlation_precision_scorer": math.Round(scorerCorrPrec*1000) / 1000,
		"dimensions": map[string]interface{}{
			"vulnerability":       math.Round(vulnScore*100) / 100,
			"capability_exposure": math.Round(capScore*100) / 100,
			"attack_path":         math.Round(pathScore*100) / 100,
			"rbac_policy":         math.Round(rbacScore*100) / 100,
			"runtime_threat":      math.Round(runtimeScore*100) / 100,
			"exposure":            math.Round(exposureScore*100) / 100,
			"blast_radius":        math.Round(blastScore*100) / 100,
		},
		"toxic_combos":                   toxicCombos,
		"time_decay":                     math.Round(timeDecay*100) / 100,
		"runtime_signals_24h":            runtimeSignalEvents24h,
		"runtime_signals_5m":             runtimeUniqueSignalTypes5m,
		"runtime_events_5m":              runtimeBurstEvents5m,
		"runtime_derived_capabilities":   runtimeDerivedCaps,
		"runtime_satisfied_requirements": runtimeSatisfiedRequirements(runtimeEvents24h, runtimeSignals),
		"sa_token_satisfaction_weight":   math.Round(saTokSatW*1000) / 1000,
		"path_mitre_relevance_count":      len(pathMitreSet),
		"runtime_temporal_coherence": map[string]interface{}{
			"sequence_bonus":          temporalCoherence.SequenceBonus,
			"disorder_penalty":        temporalCoherence.DisorderPenalty,
			"coherence_multiplier":    temporalCoherence.CoherenceMultiplier,
			"pairing_window_exceeded": temporalCoherence.PairingWindowExceeded,
			"pairing_gap_seconds":     temporalCoherence.PairingGapSeconds,
		},
		"capability_validation_confidence": math.Round(capValConf*1000) / 1000,
		"runtime_confidence_score":         math.Round(runtimeConfSc*1000) / 1000,
		"drift_score_raw":                  math.Round(trendDelta*100) / 100,
		"corroboration_k8s_api":            corroborationK8s,
		"explainability": map[string]interface{}{
			"runtime_threat": map[string]interface{}{
				"matched_runtime_signals": pathInf.MatchedRuntimeSignals,
				"runtime_step_progress":   pathInf.RuntimeStepProgress,
				"k8s_api_corroboration":   corroborationK8s,
				"satisfied_requirements":  runtimeSatisfiedRequirements(runtimeEvents24h, runtimeSignals),
			},
		},
		"insights_count":     len(insights),
		"capabilities_count": len(caps),
		"attack_paths_count": len(attackPaths),
		"attack_path_reasoning": map[string]interface{}{
			"max_strength":            pathInf.MaxStrength,
			"avg_strength":            pathInf.AvgStrength,
			"progress":                pathInf.Progress,
			"reaches_node":            pathInf.ReachesNode,
			"class_counts":            pathInf.ClassCounts,
			"e_inject_norm":           pathInf.EInjectNorm,
			"r_inject_norm":           pathInf.RInjectNorm,
			"i_inject_norm":           pathInf.IInjectNorm,
			"graph_based_e":           pathInf.GraphBasedE,
			"runtime_based_e":         pathInf.RuntimeBasedE,
			"runtime_decay":           pathInf.RuntimeDecay,
			"diversity_factor":        pathInf.DiversityFactor,
			"critical_hit_count":      pathInf.CriticalHitCount,
			"e_inject_points":         pathInf.EInjectPoints,
			"r_inject_points":         pathInf.RInjectPoints,
			"i_inject_points":         pathInf.IInjectPoints,
			"applied_path_count":      pathInf.AppliedPathCount,
			"runtime_step_progress":   pathInf.RuntimeStepProgress,
			"progress_raw":            pathInf.ProgressRaw,
			"progress_saturated":      pathInf.ProgressSaturated,
			"inject_scale":            pathInf.InjectScale,
			"matched_runtime_signals": pathInf.MatchedRuntimeSignals,
		},
		"path_influence": map[string]interface{}{
			"max_strength":       pathInf.MaxStrength,
			"progress_raw":       pathInf.ProgressRaw,
			"progress_saturated": pathInf.ProgressSaturated,
			"inject_scale":       pathInf.InjectScale,
			"runtime_decay":      pathInf.RuntimeDecay,
			"diversity_factor":   pathInf.DiversityFactor,
			"critical_hit_count": pathInf.CriticalHitCount,
			"axis_injection": map[string]interface{}{
				"exploitability": pathInf.EInjectNorm,
				"impact":         pathInf.IInjectNorm,
				"reachability":   pathInf.RInjectNorm,
			},
		},
		"mapping_version": mappingMeta,
		"temporal":        temporal,
		"context": map[string]interface{}{
			"tier":                 ctxInfo.Tier,
			"base_multiplier":      ctxInfo.BaseMultiplier,
			"internet_facing":      ctxInfo.InternetFacing,
			"system_namespace":     ctxInfo.SystemNamespace,
			"effective_multiplier": ctxInfo.EffectiveMultiplier,
		},
		"aggregation": map[string]interface{}{
			"mode":                   "v3_triaxial",
			"final_formula":          agg.FinalFormula,
			"total_score_raw":        agg.TotalScoreRaw,
			"base_risk":              agg.BaseRisk,
			"threat_core":            agg.ThreatCore,
			"threat_amplifier_raw":   agg.ThreatAmplifierRaw,
			"threat_amplifier":       agg.ThreatAmplifier,
			"combo_threat_boost":     agg.ComboThreatBoost,
			"max_threat_amplifier":   agg.MaxThreatAmplifier,
			"category_sums":          agg.CategorySums,
			"category_caps":          agg.CategoryCaps,
			"risk_factors":           agg.Factors,
			"interaction_combos":     agg.InteractionCombos,
			"axes":                   agg.Axes,
			"combo_amplifier":        agg.ComboAmplifier,
			"overrides_applied":      agg.OverridesApplied,
			"context_multiplier":     agg.ContextMultiplier,
		},
	}
	for key, value := range mitreBoostMeta {
		factors[key] = value
	}
	for key, value := range runtimeThreatMeta {
		factors[key] = value
	}
	dimSum := vulnScore + capScore + pathScore + rbacScore + runtimeScore + exposureScore + blastScore
	if dimSum > 1e-6 {
		factors["runtime_influence_ratio_vs_dimensions"] = math.Round((runtimeScore/dimSum)*1000) / 1000
	}
	ratioVsTotal := 0.0
	if totalScore >= 5.0-1e-6 && totalScore > 1e-6 {
		ratioVsTotal = math.Round((runtimeScore/totalScore)*1000) / 1000
	}
	factors["runtime_influence_ratio_vs_total"] = ratioVsTotal
	if scorerCorrPrec > 0 && scorerCorrPrec < 0.35 {
		factors["alert_over_inference_risk"] = true
	}
	if ratioVsTotal > 0.42 && totalScore >= 5.0-1e-6 {
		factors["alert_runtime_dominance"] = true
	}
	if ratioVsTotal < 0.06 && len(runtimeSignals) > 4 && totalScore >= 5.0-1e-6 {
		factors["alert_silent_runtime"] = true
	}

	return &UnifiedScoreV3{
		ResourceUID:             id.ResourceUID,
		ResourceName:            resourceName,
		ResourceType:            resourceType,
		Namespace:               namespace,
		ClusterID:               id.ClusterID,
		PriorityLevel:           "",
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

func (s *UnifiedScorerV3) SaveScoreV3ForIdentity(ctx context.Context, id resourceidentity.Identity, score *UnifiedScoreV3) error {
	if err := id.Validate(); err != nil {
		return err
	}
	if score == nil || score.ClusterID != id.ClusterID || score.ResourceUID != id.ResourceUID {
		return fmt.Errorf("risk score identity mismatch")
	}
	return s.SaveScoreV3(ctx, score)
}

func (s *UnifiedScorerV3) ScheduleUnifiedScoreCalculationForIdentity(id resourceidentity.Identity) {
	if id.Validate() != nil {
		return
	}
	identityKey, err := id.Key()
	if err != nil {
		return
	}
	go func() {
		ctx := context.Background()
		score, err := s.CalculateScoreV3ForIdentity(ctx, id)
		if err != nil {
			log.Printf("[UnifiedScorerV3] scoped calculation failed for %s: %v", identityKey, err)
			return
		}
		if err := s.SaveScoreV3ForIdentity(ctx, id, score); err != nil {
			log.Printf("[UnifiedScorerV3] scoped save failed for %s: %v", identityKey, err)
			return
		}
		log.Printf("[UnifiedScorerV3] scoped score updated for %s total=%.1f", identityKey, score.TotalScore)
	}()
}
