package rep

import (
	"context"
	"encoding/json"
	"log"
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
}

type ProcessResult struct {
	Signal       string
	Mitre        string
	BaseScore    int
	CapabilityID string
	Severity     string
}

func ProcessRuntimeEvent(ctx context.Context, db *gorm.DB, input RuntimeEventInput) (*ProcessResult, error) {
	// Step 1: Create raw runtime event
	event := models.RuntimeEvent{
		PodUID:     input.PodUID,
		Namespace:  input.Namespace,
		Syscall:    input.Syscall,
		TargetPath: input.TargetPath,
		Capability: input.Capability,
	}
	if input.Timestamp != nil {
		event.CreatedAt = *input.Timestamp
	} else {
		event.CreatedAt = time.Now()
	}
	if err := db.WithContext(ctx).Create(&event).Error; err != nil {
		return nil, err
	}

	// Step 2: Convert to semantic signal using SignalAdapter
	signalAdapter := NewSignalAdapter(db)
	if err := signalAdapter.AdaptAndPersist(ctx, &event); err != nil {
		log.Printf("[REP] Failed to adapt event to signal: %v", err)
		// Continue even if signal adaptation fails
	}

	// Step 3: Get signal type for promotion
	signal, mitre, baseScore := classifySignal(input.Syscall, input.TargetPath, input.Capability, db, ctx, input.PodUID)
	if signal == "" {
		return nil, nil
	}

	// Step 4: Update risk profile
	_, namespace := ensurePodRiskProfile(ctx, db, input.PodUID, input.Namespace)
	runtimeScore := upsertRuntimeScore(ctx, db, input.PodUID, namespace, baseScore)

	// Step 5: Use CSC to promote capabilities based on signal
	csc := capability.NewCapabilityStateController(db)
	capabilityID, severity := scoreToCapability(runtimeScore)
	if capabilityID != "" {
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

func classifySignal(syscall, target, capabilityName string, db *gorm.DB, ctx context.Context, podUID string) (string, string, int) {
	syscall = strings.ToLower(strings.TrimSpace(syscall))
	target = strings.TrimSpace(target)
	capabilityName = strings.ToUpper(strings.TrimSpace(capabilityName))

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
	if isSuspiciousProcessSnapshotExec(syscall, capabilityName, target) {
		return "SUSPICIOUS_EXEC_FROM_SNAPSHOT", "T1059", 45
	}
	if isNetworkQueueSpike(syscall, capabilityName) {
		return "NETWORK_QUEUE_ANOMALY", "T1046", 35
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
	t := strings.ToLower(strings.TrimSpace(target))
	if t == "" {
		return false
	}
	// Heuristic bucket for high-risk tooling frequently used in runtime abuse.
	keywords := []string{
		"bash", "sh", "nc", "netcat", "ncat", "socat",
		"curl", "wget", "python", "perl", "ruby",
	}
	for _, k := range keywords {
		if strings.Contains(t, k) {
			return true
		}
	}
	// Direct execution from writable locations is suspicious.
	return strings.HasPrefix(t, "/tmp/") || strings.HasPrefix(t, "/dev/shm/")
}

func isNetworkQueueSpike(syscall, capabilityName string) bool {
	if strings.ToLower(strings.TrimSpace(syscall)) != "connect" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(capabilityName), "NETWORK_TXRX_QUEUE_SPIKE")
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
