package rep

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fortuna/core/pkg/capability"
	"github.com/fortuna/core/pkg/models"
)

type RuntimeEventInput struct {
	PodUID     string
	Namespace  string
	Syscall    string
	TargetPath string
	Capability string
	Timestamp  *time.Time
	// Canonical contract fields (P0.1): optional, best-effort.
	EventID         string
	ObservedAt      *time.Time
	IngestedAt      *time.Time
	ResolutionState string
	SourceKind      string
	SourceSensorID  string
	SourceRule      string
	PayloadJSON     string
	PayloadHash     string
	// Optional enrichment for R9 correlation/richness.
	PodName        string
	NodeName       string
	Runtime        string
	EventType      string
	Signal         string
	MitreTechnique string
	Severity       string
	// Confidence is required (must be > 0). Sensors must supply explicit confidence.
	Confidence float64
}

type ProcessResult struct {
	Signal       string
	Mitre        string
	BaseScore    int
	CapabilityID string
	Severity     string
}

func ProcessRuntimeEvent(ctx context.Context, db *gorm.DB, input RuntimeEventInput) (*ProcessResult, error) {
	if input.Confidence <= 0 {
		return nil, fmt.Errorf("rep: runtime event confidence must be > 0")
	}

	payloadJSON := strings.TrimSpace(input.PayloadJSON)
	if payloadJSON == "" || !json.Valid([]byte(payloadJSON)) {
		payloadJSON = "{}"
	}

	// Step 1: Create raw runtime event
	event := models.RuntimeEvent{
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
		PodUID:          input.PodUID,
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

	// Step 1.5 (P0): extract normalized behavior facts from raw runtime event.
	// This runs in parallel with existing REP v1 signal path to keep compatibility.
	facts, err := extractAndPersistBehaviorFacts(ctx, db, &event)
	if err != nil {
		log.Printf("[REP] Failed to extract behavior facts: %v", err)
		// non-fatal: keep existing runtime_signals pipeline alive
	}
	// Step 1.5b (P0.2+/REP-B): facts -> synthesized semantic signals (persist path).
	// This fills missing signals without double-counting within the current day window.
	if facts != nil && len(facts) > 0 {
		cands := synthesizeSignalsFromFacts(facts)
		if err2 := persistSynthesizedSignalsFromFacts(ctx, db, &event, facts, cands); err2 != nil {
			log.Printf("[REP] Failed to persist signals from facts: %v", err2)
		}
	}
	// Step 1.6 (P0): REP-C minimal correlator (stateful incidents).
	if err := correlateAndPersistRuntimeIncidents(ctx, db, &event, facts); err != nil {
		log.Printf("[REP] Failed to correlate runtime incidents: %v", err)
	}

	// Step 2: Convert to semantic signal using SignalAdapter
	signalAdapter := NewSignalAdapter(db)
	if err := signalAdapter.AdaptAndPersist(ctx, &event); err != nil {
		log.Printf("[REP] Failed to adapt event to signal: %v", err)
		// Continue even if signal adaptation fails
	}

	// Step 3: Get signal type for promotion
	signal, mitre, baseScore := classifySignal(input.Syscall, input.TargetPath, input.Capability, input.Runtime, input.SourceRule, db, ctx, input.PodUID)
	if signal == "" {
		// Compare-only mode for REP v2: observe where fact-based synthesis sees signals while legacy path misses.
		if cands := synthesizeSignalsFromFacts(facts); len(cands) > 0 {
			log.Printf("[REPv2-compare] legacy=no-match pod_uid=%s facts=%d synthesized=%v",
				input.PodUID, len(facts), signalTypes(cands))
		}
		return nil, nil
	}

	// Compare-only mode for REP v2: no persistence yet, just parity visibility.
	if cands := synthesizeSignalsFromFacts(facts); len(cands) > 0 {
		match := containsSignalType(cands, signal)
		if !match {
			log.Printf("[REPv2-compare] mismatch pod_uid=%s legacy=%s synthesized=%v", input.PodUID, signal, signalTypes(cands))
		}
	}

	// Step 4: Update risk profile
	_, namespace := ensurePodRiskProfile(ctx, db, input.PodUID, input.Namespace)
	runtimeScore := upsertRuntimeScore(ctx, db, input.PodUID, namespace, baseScore)

	// Step 5: Use CSC to promote capabilities based on signal
	csc := capability.NewCapabilityStateController(db)
	capabilityID, severity := scoreToCapability(runtimeScore)
	if capabilityID != "" {
		// P0.3 runtime-first capability init:
		// ensure capability exists even if no promotion_rule matches this signal yet.
		runtimeEvidence := map[string]interface{}{
			"source":        "runtime",
			"signal_type":   signal,
			"runtime_score": runtimeScore,
			"base_score":    baseScore,
			"mitre":         mitre,
		}
		if err := csc.InitializeCapability(ctx, input.PodUID, namespace, capabilityID, "ESC", severity, runtimeEvidence); err != nil {
			log.Printf("[REP] Failed to initialize runtime capability %s for pod %s: %v", capabilityID, input.PodUID, err)
		}

		// Get signal type from adapted signal (use same logic as classifySignal for now)
		signalType := signal
		if err := csc.PromoteCapability(ctx, input.PodUID, capabilityID, signalType, 0.9); err != nil {
			log.Printf("[REP] Failed to promote capability %s for pod %s: %v", capabilityID, input.PodUID, err)
			// Continue even if promotion fails
		}
	}

	return &ProcessResult{
		Signal:       signal,
		Mitre:        mitre,
		BaseScore:    baseScore,
		CapabilityID: capabilityID,
		Severity:     severity,
	}, nil
}

func containsSignalType(cands []synthesizedSignal, signal string) bool {
	for i := range cands {
		if cands[i].SignalType == signal {
			return true
		}
	}
	return false
}

func signalTypes(cands []synthesizedSignal) []string {
	out := make([]string, 0, len(cands))
	for i := range cands {
		out = append(out, cands[i].SignalType)
	}
	return out
}

func classifySignal(syscall, target, capabilityName, runtimeSource, sourceRule string, db *gorm.DB, ctx context.Context, podUID string) (string, string, int) {
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
	if isCapabilityMisuse(syscall, capabilityName, db, ctx, podUID) {
		return "CAPABILITY_MISUSE", "T1611.002", 60
	}

	// Falco: capability fields may not exist, so map exec/connect into Fortuna runtime-signals
	// via syscall + target heuristics to keep runtime YAML rules usable.
	if runtimeSource == "falco" {
		if strings.EqualFold(syscall, "execve") {
			if isSuspiciousProcessSnapshotExec(syscall, capabilityName, target) {
				return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "T1059", 45
			}
			// Falco provides evt.type=execve + proc.cmdline but not PROCESS_SNAPSHOT_DIFF capability.
			if falcoSuspiciousExecTarget(target) {
				return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "T1059", 48
			}
		}
		if strings.EqualFold(syscall, "connect") {
			t := strings.ToLower(target)
			looksNetwork := strings.Contains(t, ":") ||
				strings.Contains(t, "dst=") ||
				strings.Contains(t, "proto=") ||
				strings.Contains(t, "dport=")
			if looksNetwork {
				return "NETWORK_QUEUE_ANOMALY", "T1046", networkQueueSpikeScore(target)
			}
		}
	}

	// Falco: rule name (and generic falco.alert) after syscall heuristics — fills UNKNOWN gap.
	if runtimeSource == "falco" {
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

func isProcRootPivot(syscall, target string) bool {
	if syscall != "open" && syscall != "openat" && syscall != "stat" && syscall != "readlink" {
		return false
	}
	return strings.Contains(target, "/proc/1/root") ||
		strings.Contains(target, "/proc/self/exe") ||
		strings.Contains(target, "/proc/1/exe") ||
		strings.Contains(target, "/proc/") && strings.Contains(target, "/root") ||
		strings.Contains(target, "/proc/") && strings.Contains(target, "/exe")
}

func isFSEscapeAttempt(syscall, target string) bool {
	if syscall != "mount" && syscall != "pivot_root" {
		return false
	}
	return strings.HasPrefix(target, "/proc") ||
		strings.HasPrefix(target, "/sys") ||
		strings.HasPrefix(target, "/dev") ||
		strings.HasPrefix(target, "/run") ||
		strings.HasPrefix(target, "/var/run")
}

func isNamespaceEscape(syscall, target string) bool {
	if syscall != "setns" && syscall != "unshare" && syscall != "clone" {
		return false
	}
	return strings.Contains(target, "/proc/") && strings.Contains(target, "/ns/")
}

func isCapabilityMisuse(syscall, capName string, db *gorm.DB, ctx context.Context, podUID string) bool {
	if syscall != "mount" && syscall != "setns" && syscall != "pivot_root" {
		return false
	}
	if capName != "SYS_ADMIN" {
		return false
	}
	var pod models.Pod
	if err := db.WithContext(ctx).Where("uid = ?", podUID).First(&pod).Error; err != nil {
		return true
	}
	return !podAllowsCapability(pod.ContainerSecurityContexts, "SYS_ADMIN")
}

func isSuspiciousProcessSnapshotExec(syscall, capName, target string) bool {
	if strings.ToLower(strings.TrimSpace(syscall)) != "execve" {
		return false
	}
	if strings.ToUpper(strings.TrimSpace(capName)) != "PROCESS_SNAPSHOT_DIFF" {
		return false
	}
	return falcoSuspiciousExecTarget(target)
}

// falcoSuspiciousExecTarget mirrors SignalAdapter execve heuristics for Falco proc.cmdline targets.
func falcoSuspiciousExecTarget(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	if t == "" {
		return false
	}
	keywords := []string{
		"bash", "sh", "nc", "netcat", "ncat", "socat",
		"curl", "wget", "python", "perl", "ruby",
	}
	for _, k := range keywords {
		if strings.Contains(t, k) {
			return true
		}
	}
	return strings.HasPrefix(t, "/tmp/") || strings.HasPrefix(t, "/dev/shm/")
}

func isNetworkQueueSpike(syscall, capabilityName string) bool {
	if strings.ToLower(strings.TrimSpace(syscall)) != "connect" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(capabilityName), "NETWORK_TXRX_QUEUE_SPIKE")
}

func networkQueueSpikeScore(target string) int {
	ratio := parseTargetFloatKV(target, "ratio")
	// Default for backward compatibility when old events don't carry ratio.
	if ratio <= 0 {
		return 35
	}
	switch {
	case ratio >= 12:
		return 65
	case ratio >= 8:
		return 55
	case ratio >= 5:
		return 45
	default:
		return 35
	}
}

func parseTargetFloatKV(target, key string) float64 {
	if target == "" || key == "" {
		return 0
	}
	for _, token := range strings.Fields(target) {
		if !strings.HasPrefix(token, key+"=") {
			continue
		}
		v := strings.TrimPrefix(token, key+"=")
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

func isEBPFExecTrace(syscall, capabilityName string) bool {
	return strings.EqualFold(strings.TrimSpace(syscall), "execve") &&
		strings.EqualFold(strings.TrimSpace(capabilityName), "EBPF_EXEC_TRACE")
}

func isEBPFConnectTrace(syscall, capabilityName string) bool {
	return strings.EqualFold(strings.TrimSpace(syscall), "connect") &&
		strings.EqualFold(strings.TrimSpace(capabilityName), "EBPF_CONNECT_TRACE")
}

func podAllowsCapability(containerSecurityContexts, capName string) bool {
	if containerSecurityContexts == "" {
		return false
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(containerSecurityContexts), &m); err != nil {
		return false
	}
	for _, v := range m {
		cm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		caps, ok := cm["capabilities"].(map[string]interface{})
		if !ok {
			continue
		}
		add, ok := caps["add"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range add {
			if s, ok := c.(string); ok && strings.EqualFold(s, capName) {
				return true
			}
		}
	}
	return false
}

func ensurePodRiskProfile(ctx context.Context, db *gorm.DB, podUID, namespace string) (int, string) {
	var pod models.Pod
	if err := db.WithContext(ctx).Where("uid = ?", podUID).First(&pod).Error; err == nil {
		staticRisk := capability.ComputeStaticRisk(&pod)
		profile := models.PodRiskProfile{
			PodUID:       podUID,
			Namespace:    pod.Namespace,
			StaticRisk:   staticRisk,
			RuntimeScore: 0,
			Capabilities: pq.StringArray{},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		_ = db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pod_uid"}},
			DoUpdates: clause.AssignmentColumns([]string{"namespace", "static_risk", "updated_at"}),
		}).Create(&profile).Error
		return staticRisk, pod.Namespace
	}

	if namespace == "" {
		namespace = "default"
	}
	profile := models.PodRiskProfile{
		PodUID:       podUID,
		Namespace:    namespace,
		StaticRisk:   0,
		RuntimeScore: 0,
		Capabilities: pq.StringArray{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_ = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pod_uid"}},
		DoUpdates: clause.AssignmentColumns([]string{"namespace", "updated_at"}),
	}).Create(&profile).Error
	return 0, namespace
}

func upsertRuntimeScore(ctx context.Context, db *gorm.DB, podUID, namespace string, score int) int {
	var profile models.PodRiskProfile
	if err := db.WithContext(ctx).Where("pod_uid = ?", podUID).First(&profile).Error; err != nil {
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

	// Update capabilities array based on runtime score
	capabilityID, _ := scoreToCapability(newScore)
	if capabilityID != "" {
		capabilities := profile.Capabilities
		found := false
		for _, cap := range capabilities {
			if cap == capabilityID {
				found = true
				break
			}
		}
		if !found {
			profile.Capabilities = append(capabilities, capabilityID)
		}
	}

	_ = db.WithContext(ctx).Save(&profile).Error
	return newScore
}

func scoreToCapability(score int) (string, string) {
	if score >= 90 {
		return capability.ESC_RUNTIME_ACTIVE, "CRITICAL"
	}
	if score >= 60 {
		return capability.ESC_RUNTIME_PROBE, "HIGH"
	}
	return "", ""
}

// upsertRuntimeCapability is DEPRECATED - use CapabilityStateController.PromoteCapability() instead
// This function is kept for backward compatibility but should not be called directly
// All capability state updates should go through CSC to ensure consistency
func upsertRuntimeCapability(ctx context.Context, db *gorm.DB, podUID, namespace, capabilityID, severity, signal, mitre string, input RuntimeEventInput) error {
	log.Printf("[REP] WARNING: upsertRuntimeCapability() is deprecated, use CSC.PromoteCapability() instead")
	// This function is no longer used - capability promotion is handled by CSC in ProcessRuntimeEvent()
	return nil
}
