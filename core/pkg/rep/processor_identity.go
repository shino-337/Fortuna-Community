package rep

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/capability"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProcessRuntimeEventForIdentity is the production multi-cluster REP path. Every
// Pod-owned read/write uses the explicit canonical identity supplied by the
// authenticated runtime ownership boundary.
func ProcessRuntimeEventForIdentity(ctx context.Context, db *gorm.DB, id resourceidentity.Identity, input RuntimeEventInput) (*ProcessResult, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	if input.PodUID != id.ResourceUID {
		return nil, fmt.Errorf("rep: runtime input pod uid does not match canonical identity")
	}
	if input.Confidence <= 0 {
		return nil, fmt.Errorf("rep: runtime event confidence must be > 0")
	}

	payloadJSON := strings.TrimSpace(input.PayloadJSON)
	if payloadJSON == "" || !json.Valid([]byte(payloadJSON)) {
		payloadJSON = "{}"
	}
	event := models.RuntimeEvent{
		ClusterID:       id.ClusterID,
		EventID:         strings.TrimSpace(input.EventID),
		ObservedAt:      input.ObservedAt,
		IngestedAt:      input.IngestedAt,
		ResolutionState: strings.TrimSpace(input.ResolutionState),
		SourceKind:      strings.TrimSpace(input.SourceKind),
		SourceSensorID:  strings.TrimSpace(input.SourceSensorID),
		SourceRule:      strings.TrimSpace(input.SourceRule),
		PayloadJSON:     payloadJSON,
		PayloadHash:     strings.TrimSpace(input.PayloadHash),
		PodName:         input.PodName,
		PodUID:          id.ResourceUID,
		Namespace:       input.Namespace,
		NodeName:        input.NodeName,
		Runtime:         input.Runtime,
		EventType:       input.EventType,
		Signal:          input.Signal,
		Mitre:           input.MitreTechnique,
		Severity:        input.Severity,
		Confidence:      input.Confidence,
		Syscall:         input.Syscall,
		TargetPath:      input.TargetPath,
		Capability:      input.Capability,
	}
	if input.Timestamp != nil {
		event.CreatedAt = *input.Timestamp
	} else {
		event.CreatedAt = time.Now()
	}
	if err := db.WithContext(ctx).Create(&event).Error; err != nil {
		return nil, err
	}

	facts, err := extractAndPersistBehaviorFacts(ctx, db, &event)
	if err != nil {
		log.Printf("[REP] scoped fact extraction failed for %s/%s: %v", id.ClusterID, id.ResourceUID, err)
	}
	if len(facts) > 0 {
		if err := persistSynthesizedSignalsFromFacts(ctx, db, &event, facts, synthesizeSignalsFromFacts(facts)); err != nil {
			log.Printf("[REP] scoped synthesized signal persistence failed: %v", err)
		}
	}
	if err := correlateAndPersistRuntimeIncidents(ctx, db, &event, facts); err != nil {
		log.Printf("[REP] scoped runtime incident correlation failed: %v", err)
	}
	if err := NewSignalAdapter(db).AdaptAndPersist(ctx, &event); err != nil {
		log.Printf("[REP] scoped signal adaptation failed: %v", err)
	}

	signal, mitre, baseScore := classifySignalForIdentity(input.Syscall, input.TargetPath, input.Capability, input.Runtime, input.SourceRule, db, ctx, id)
	if signal == "" {
		return nil, nil
	}
	_, namespace := ensurePodRiskProfileForIdentity(ctx, db, id, input.Namespace)
	runtimeScore := upsertRuntimeScoreForIdentity(ctx, db, id, namespace, baseScore)

	csc := capability.NewCapabilityStateController(db)
	capabilityID, severity := scoreToCapability(runtimeScore)
	if capabilityID != "" {
		runtimeEvidence := map[string]interface{}{
			"source":        "runtime",
			"signal_type":   signal,
			"runtime_score": runtimeScore,
			"base_score":    baseScore,
			"mitre":         mitre,
		}
		if err := csc.InitializeCapabilityForIdentity(ctx, id, namespace, capabilityID, "ESC", severity, runtimeEvidence); err != nil {
			log.Printf("[REP] scoped capability initialization failed for %s/%s: %v", id.ClusterID, id.ResourceUID, err)
		}
		if err := csc.PromoteCapabilityForIdentity(ctx, id, capabilityID, signal, input.Confidence); err != nil {
			log.Printf("[REP] scoped capability promotion failed for %s/%s: %v", id.ClusterID, id.ResourceUID, err)
		}
	}

	return &ProcessResult{Signal: signal, Mitre: mitre, BaseScore: baseScore, CapabilityID: capabilityID, Severity: severity}, nil
}

func classifySignalForIdentity(syscall, target, capabilityName, runtimeSource, sourceRule string, db *gorm.DB, ctx context.Context, id resourceidentity.Identity) (string, string, int) {
	syscall = strings.ToLower(strings.TrimSpace(syscall))
	target = strings.TrimSpace(target)
	capabilityName = strings.ToUpper(strings.TrimSpace(capabilityName))
	runtimeSource = strings.ToLower(strings.TrimSpace(runtimeSource))
	if isProcRootPivot(syscall, target) {
		return "PROC_ROOT_PIVOT", "T1611.001", 90
	}
	if isFSEscapeAttempt(syscall, target) {
		return "FS_ESCAPE_ATTEMPT", "T1610", 95
	}
	if isNamespaceEscape(syscall, target) {
		return "NAMESPACE_ESCAPE", "T1055", 85
	}
	if isCapabilityMisuseForIdentity(syscall, capabilityName, db, ctx, id) {
		return "CAPABILITY_MISUSE", "T1611.002", 60
	}
	if runtimeSource == "falco" {
		if syscall == "execve" && falcoSuspiciousExecTarget(target) {
			return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "T1059", 48
		}
		if syscall == "connect" {
			t := strings.ToLower(target)
			if strings.Contains(t, ":") || strings.Contains(t, "dst=") || strings.Contains(t, "proto=") || strings.Contains(t, "dport=") {
				return "NETWORK_QUEUE_ANOMALY", "T1046", networkQueueSpikeScore(target)
			}
		}
		if sig, ok := ClassifyFalcoRuleToSignal(sourceRule, syscall); ok {
			return sig.SignalType, sig.Mitre, sig.BaseScore
		}
	}
	if isSuspiciousProcessSnapshotExec(syscall, capabilityName, target) {
		return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "T1059", 45
	}
	if isNetworkQueueSpike(syscall, capabilityName) {
		return "NETWORK_QUEUE_ANOMALY", "T1046", networkQueueSpikeScore(target)
	}
	if isEBPFExecTrace(syscall, capabilityName) {
		return "EBPF_EXEC_ACTIVITY", "T1059", 40
	}
	if isEBPFConnectTrace(syscall, capabilityName) {
		return "EBPF_CONNECT_ACTIVITY", "T1046", 30
	}
	return "", "", 0
}

func isCapabilityMisuseForIdentity(syscall, capName string, db *gorm.DB, ctx context.Context, id resourceidentity.Identity) bool {
	if syscall != "mount" && syscall != "setns" && syscall != "pivot_root" {
		return false
	}
	if capName != "SYS_ADMIN" {
		return false
	}
	var pod models.Pod
	if err := db.WithContext(ctx).
		Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", id.ClusterID, id.ResourceUID).
		First(&pod).Error; err != nil {
		return true
	}
	return !podAllowsCapability(pod.ContainerSecurityContexts, "SYS_ADMIN")
}

func ensurePodRiskProfileForIdentity(ctx context.Context, db *gorm.DB, id resourceidentity.Identity, namespace string) (int, string) {
	staticRisk := 0
	var pod models.Pod
	if err := db.WithContext(ctx).
		Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", id.ClusterID, id.ResourceUID).
		First(&pod).Error; err == nil {
		staticRisk = capability.ComputeStaticRisk(&pod)
		namespace = pod.Namespace
	}
	if namespace == "" {
		namespace = "default"
	}
	now := time.Now()
	profile := models.PodRiskProfile{
		ClusterID: id.ClusterID, PodUID: id.ResourceUID, Namespace: namespace,
		StaticRisk: staticRisk, RuntimeScore: 0, Capabilities: pq.StringArray{},
		CreatedAt: now, UpdatedAt: now,
	}
	_ = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cluster_id"}, {Name: "pod_uid"}},
		DoUpdates: clause.AssignmentColumns([]string{"namespace", "static_risk", "updated_at"}),
	}).Create(&profile).Error
	return staticRisk, namespace
}

func upsertRuntimeScoreForIdentity(ctx context.Context, db *gorm.DB, id resourceidentity.Identity, namespace string, score int) int {
	var profile models.PodRiskProfile
	if err := db.WithContext(ctx).
		Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, id.ResourceUID).
		First(&profile).Error; err != nil {
		return score
	}
	newScore := profile.RuntimeScore + score
	if newScore > 100 {
		newScore = 100
	}
	profile.RuntimeScore = newScore
	profile.Namespace = namespace
	now := time.Now()
	profile.LastEventAt = &now
	profile.UpdatedAt = now
	capabilityID, _ := scoreToCapability(newScore)
	if capabilityID != "" {
		found := false
		for _, current := range profile.Capabilities {
			if current == capabilityID {
				found = true
				break
			}
		}
		if !found {
			profile.Capabilities = append(profile.Capabilities, capabilityID)
		}
	}
	_ = db.WithContext(ctx).Save(&profile).Error
	return newScore
}
