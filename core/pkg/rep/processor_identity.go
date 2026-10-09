package rep

import (
	"context"
	"encoding/json"
	"fmt"
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
	sourceRecordID := strings.TrimSpace(input.SourceRecordID)
	if sourceRecordID == "" || len(sourceRecordID) > 64 {
		return nil, fmt.Errorf("rep: physical source record id is required")
	}

	payloadJSON := strings.TrimSpace(input.PayloadJSON)
	if payloadJSON == "" || !json.Valid([]byte(payloadJSON)) {
		payloadJSON = "{}"
	}
	event := models.RuntimeEvent{
		ClusterID:       id.ClusterID,
		AgentID:         strings.TrimSpace(input.AgentID),
		EventID:         strings.TrimSpace(input.EventID),
		SourceRecordID:  sourceRecordID,
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

	var result *ProcessResult
	scheduleAttackPathRebuild := false
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// The runtime_events insert is the atomic idempotency claim. PostgreSQL
		// serializes concurrent submissions on the partial unique index
		// {cluster_id, agent_id, source_record_id}. The claim and every downstream effect
		// share this transaction, so rollback releases the claim for a later retry.
		insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
		if insert.Error != nil {
			return insert.Error
		}
		if insert.RowsAffected == 0 {
			var existing models.RuntimeEvent
			if err := tx.Where("cluster_id = ? AND agent_id = ? AND source_record_id = ?", id.ClusterID, event.AgentID, sourceRecordID).
				First(&existing).Error; err != nil {
				return fmt.Errorf("rep: source-record conflict without existing event: %w", err)
			}
			if !runtimeEventReplayMatches(existing, event) {
				return ErrRuntimeSourceRecordConflict
			}
			result = &ProcessResult{Duplicate: true}
			return nil
		}

		facts, err := extractAndPersistBehaviorFacts(ctx, tx, &event)
		if err != nil {
			return fmt.Errorf("rep: persist behavior facts: %w", err)
		}
		if len(facts) > 0 {
			if err := persistSynthesizedSignalsFromFacts(ctx, tx, &event, facts, synthesizeSignalsFromFacts(facts)); err != nil {
				return fmt.Errorf("rep: persist synthesized signals: %w", err)
			}
		}
		if err := correlateAndPersistRuntimeIncidents(ctx, tx, &event, facts); err != nil {
			return fmt.Errorf("rep: correlate runtime incidents: %w", err)
		}
		if err := NewSignalAdapter(tx).AdaptAndPersist(ctx, &event); err != nil {
			return fmt.Errorf("rep: adapt runtime signal: %w", err)
		}

		signal, mitre, baseScore := classifySignalForIdentity(input.Syscall, input.TargetPath, input.Capability, input.Runtime, input.SourceRule, tx, ctx, id)
		if signal == "" {
			result = &ProcessResult{}
			return nil
		}
		_, namespace, err := ensurePodRiskProfileForIdentity(ctx, tx, id, input.Namespace)
		if err != nil {
			return err
		}
		runtimeScore, err := upsertRuntimeScoreForIdentity(ctx, tx, id, namespace, baseScore)
		if err != nil {
			return err
		}

		csc := capability.NewCapabilityStateController(tx)
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
				return fmt.Errorf("rep: initialize runtime capability: %w", err)
			}
			promoted, err := csc.PromoteCapabilityForIdentityTx(ctx, id, capabilityID, signal, input.Confidence)
			if err != nil {
				return fmt.Errorf("rep: promote runtime capability: %w", err)
			}
			if promoted {
				scheduleAttackPathRebuild = true
			}
		}

		result = &ProcessResult{Signal: signal, Mitre: mitre, BaseScore: baseScore, CapabilityID: capabilityID, Severity: severity}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if scheduleAttackPathRebuild {
		// Never hand an async goroutine the transaction handle: it becomes invalid
		// immediately after commit. Use the base DB only after atomic REP commit.
		capability.ScheduleAttackPathRebuildForIdentity(db, id)
	}
	return result, nil
}

func runtimeEventReplayMatches(existing, candidate models.RuntimeEvent) bool {
	// Ingest/processing timestamps and the older second-granularity EventID are
	// deliberately excluded. A physical JSONL record may be reconstructed on a
	// later poll/restart and receive new transport timestamps, while its source
	// record identity and semantic payload remain unchanged.
	return existing.ClusterID == candidate.ClusterID &&
		existing.AgentID == candidate.AgentID &&
		existing.SourceRecordID == candidate.SourceRecordID &&
		existing.PayloadHash == candidate.PayloadHash &&
		existing.PodUID == candidate.PodUID &&
		existing.Namespace == candidate.Namespace &&
		existing.NodeName == candidate.NodeName &&
		existing.SourceKind == candidate.SourceKind &&
		existing.SourceSensorID == candidate.SourceSensorID &&
		existing.SourceRule == candidate.SourceRule &&
		existing.Runtime == candidate.Runtime &&
		existing.EventType == candidate.EventType &&
		existing.Signal == candidate.Signal &&
		existing.Mitre == candidate.Mitre &&
		existing.Severity == candidate.Severity &&
		existing.Syscall == candidate.Syscall &&
		existing.TargetPath == candidate.TargetPath &&
		existing.Capability == candidate.Capability &&
		existing.Confidence == candidate.Confidence
}

func classifySignalForIdentity(syscall, target, capabilityName, runtimeSource, sourceRule string, db *gorm.DB, ctx context.Context, id resourceidentity.Identity) (string, string, int) {
	syscall = strings.ToLower(strings.TrimSpace(syscall))
	target = strings.TrimSpace(target)
	capabilityName = strings.ToUpper(strings.TrimSpace(capabilityName))
	runtimeSource = strings.ToLower(strings.TrimSpace(runtimeSource))
	if isProcRootPivot(syscall, target) {
		return "PROC_ROOT_PIVOT", "T1611", 90
	}
	if isFSEscapeAttempt(syscall, target) {
		return "FS_ESCAPE_ATTEMPT", "T1611", 95
	}
	if isNamespaceEscape(syscall, target) {
		return "NAMESPACE_ESCAPE", "T1055", 85
	}
	if isCapabilityMisuseForIdentity(syscall, capabilityName, db, ctx, id) {
		return "CAPABILITY_MISUSE", "T1611", 60
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

func ensurePodRiskProfileForIdentity(ctx context.Context, db *gorm.DB, id resourceidentity.Identity, namespace string) (int, string, error) {
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
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cluster_id"}, {Name: "pod_uid"}},
		DoUpdates: clause.AssignmentColumns([]string{"namespace", "static_risk", "updated_at"}),
	}).Create(&profile).Error; err != nil {
		return 0, namespace, fmt.Errorf("rep: ensure pod risk profile: %w", err)
	}
	return staticRisk, namespace, nil
}

func upsertRuntimeScoreForIdentity(ctx context.Context, db *gorm.DB, id resourceidentity.Identity, namespace string, score int) (int, error) {
	var profile models.PodRiskProfile
	if err := db.WithContext(ctx).
		Where("cluster_id = ? AND pod_uid = ?", id.ClusterID, id.ResourceUID).
		First(&profile).Error; err != nil {
		return 0, fmt.Errorf("rep: load pod risk profile: %w", err)
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
	if err := db.WithContext(ctx).Save(&profile).Error; err != nil {
		return 0, fmt.Errorf("rep: update runtime score: %w", err)
	}
	return newScore, nil
}
