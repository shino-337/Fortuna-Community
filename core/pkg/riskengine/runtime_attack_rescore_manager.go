package riskengine

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
)

// RuntimeAttackRescoreManager debounces "attack-like" runtime events (Falco, etc.)
// and triggers:
//   1) runtime-behavior insight evaluation for the affected pod (YAML runtime rules)
//   2) unified V3 risk score recomputation
//
// Motivation: runtime ingest persists runtime_events/runtime_signals but does not
// naturally re-trigger Pod risk evaluation (normalized inventory may be unchanged),
// which can leave risk_scores stale during an active attack.
//
// This manager is intentionally in-memory (per-Core replica). Writes are idempotent
// (insights upsert + risk_scores upsert), so duplicates across replicas are safe.
type RuntimeAttackRescoreManager struct {
	db *gorm.DB

	enabled       bool
	debounce      time.Duration
	minSeverity   int
	maxBatchPods  int
	yamlEngine    *YAMLEngine
	insightMgr    *InsightManager
	rulesDir      string

	mu    sync.Mutex
	items map[string]*runtimeAttackItem
}

type runtimeAttackItem struct {
	timer       *time.Timer
	firstSeenAt time.Time
	lastSeenAt  time.Time
	maxSeverity int
	eventCount  int
}

// RuntimeEventMeta is a minimal, stable contract for deciding whether a runtime
// event should trigger a rescore.
type RuntimeEventMeta struct {
	PodUID          string
	Runtime         string
	SourceKind      string
	SourceRule      string
	Syscall         string
	Severity        string
	ResolutionState string
	ObservedAt      *time.Time
}

func NewRuntimeAttackRescoreManager(db *gorm.DB) *RuntimeAttackRescoreManager {
	m := &RuntimeAttackRescoreManager{
		db:          db,
		enabled:     envBool("RUNTIME_ATTACK_RESCORE_ENABLED", true),
		debounce:    envDuration("RUNTIME_ATTACK_RESCORE_DEBOUNCE", 30*time.Second),
		minSeverity: severityRank(envString("RUNTIME_ATTACK_RESCORE_MIN_SEVERITY", "medium")),
		maxBatchPods: func() int {
			n := envInt("RUNTIME_ATTACK_RESCORE_MAX_BATCH_PODS", 50)
			if n <= 0 {
				return 50
			}
			return n
		}(),
		items:     make(map[string]*runtimeAttackItem),
		insightMgr: NewInsightManager(db),
	}

	// Best-effort: create YAML engine once so we can evaluate runtime-behavior rules
	// during attack windows (even if pod inventory doesn't change).
	m.rulesDir = resolveRulesDirectory()
	if m.rulesDir != "" && db != nil {
		if ye, err := NewYAMLEngine(db, m.rulesDir); err == nil {
			m.yamlEngine = ye
			log.Printf("[RuntimeAttackRescore] YAML engine enabled rulesDir=%q rules=%d", m.rulesDir, len(ye.GetRules()))
		} else {
			log.Printf("[RuntimeAttackRescore] YAML engine unavailable rulesDir=%q: %v", m.rulesDir, err)
		}
	} else {
		log.Printf("[RuntimeAttackRescore] YAML engine disabled (rulesDir=%q dbNil=%v)", m.rulesDir, db == nil)
	}

	return m
}

// Notify observes a runtime event and schedules a debounced rescore when the
// event meets the "attack-like" predicate.
func (m *RuntimeAttackRescoreManager) Notify(meta RuntimeEventMeta) {
	if m == nil || !m.enabled || m.db == nil {
		return
	}
	podUID := strings.TrimSpace(meta.PodUID)
	if podUID == "" {
		return
	}
	if !m.shouldTrigger(meta) {
		return
	}

	delay := m.debounce
	if delay < 0 {
		delay = 0
	}
	now := time.Now()

	m.mu.Lock()
	item, ok := m.items[podUID]
	if !ok {
		item = &runtimeAttackItem{firstSeenAt: now}
		m.items[podUID] = item
	}
	item.lastSeenAt = now
	item.eventCount++
	item.maxSeverity = max(item.maxSeverity, severityRank(meta.Severity))

	if item.timer == nil {
		item.timer = time.AfterFunc(delay, func() { m.flush(podUID) })
	} else {
		item.timer.Reset(delay)
	}
	m.mu.Unlock()
}

func (m *RuntimeAttackRescoreManager) flush(podUID string) {
	if m == nil || m.db == nil {
		return
	}
	podUID = strings.TrimSpace(podUID)
	if podUID == "" {
		return
	}

	m.mu.Lock()
	item := m.items[podUID]
	delete(m.items, podUID)
	m.mu.Unlock()
	if item == nil {
		return
	}

	ctx := context.Background()
	// 1) Best-effort: create/update runtime-behavior insights from latest runtime_signals/runtime state.
	_ = m.evaluateAndUpsertRuntimeInsights(ctx, podUID)

	// 2) Always recompute unified score so runtime_signals are reflected even if no YAML insight fired.
	risk.NewUnifiedScorerV3(m.db).ScheduleUnifiedScoreCalculation(podUID)

	log.Printf("[RuntimeAttackRescore] flushed pod_uid=%s events=%d max_severity=%d window=%s",
		podUID, item.eventCount, item.maxSeverity, time.Since(item.firstSeenAt).Truncate(time.Second))
}

func (m *RuntimeAttackRescoreManager) shouldTrigger(meta RuntimeEventMeta) bool {
	// Ignore "resolved" events by default (Falco correlation tags may emit them).
	if strings.EqualFold(strings.TrimSpace(meta.ResolutionState), "resolved") {
		return false
	}

	runtimeSrc := strings.ToLower(strings.TrimSpace(meta.Runtime))
	sourceKind := strings.ToLower(strings.TrimSpace(meta.SourceKind))
	// Default scope: Falco only (attack-like runtime detection).
	if runtimeSrc != "falco" && sourceKind != "falco" {
		return false
	}

	// Any named Falco rule opens a debounced rescore window so low Falco priorities
	// (e.g. notice→low) still refresh V3 after taxonomy-backed signals land.
	if strings.TrimSpace(meta.SourceRule) != "" {
		return true
	}

	sev := severityRank(meta.Severity)
	if sev >= m.minSeverity {
		return true
	}

	// If severity is missing/low, only trigger on escape-class syscall hints.
	switch strings.ToLower(strings.TrimSpace(meta.Syscall)) {
	case "setns", "unshare", "pivot_root", "mount":
		return true
	default:
		return false
	}
}

func (m *RuntimeAttackRescoreManager) evaluateAndUpsertRuntimeInsights(ctx context.Context, podUID string) error {
	if m == nil || m.db == nil || m.yamlEngine == nil || m.insightMgr == nil {
		return nil
	}

	var pod models.Pod
	if err := m.db.WithContext(ctx).
		Where("uid = ? AND deleted_at IS NULL", podUID).
		First(&pod).Error; err != nil {
		return nil
	}

	normalized, err := normalizedPodForRuleEval(&pod)
	if err != nil {
		return err
	}

	insights, err := m.yamlEngine.EvaluatePodRuntimeOnly(ctx, normalized)
	if err != nil {
		return err
	}
	if len(insights) == 0 {
		return nil
	}

	// Batch upsert reduces chatter and schedules a single score calc per resource UID.
	return m.insightMgr.BatchCreateOrUpdateInsights(insights)
}

func normalizedPodForRuleEval(pod *models.Pod) (map[string]interface{}, error) {
	if pod == nil {
		return nil, nil
	}

	// Keep containers array if already stored; otherwise default empty.
	var containers interface{} = []interface{}{}
	if strings.TrimSpace(pod.Containers) != "" {
		_ = json.Unmarshal([]byte(pod.Containers), &containers)
	}

	k8sPod := map[string]interface{}{
		"kind":       "Pod",
		"apiVersion": "v1",
		"metadata": map[string]interface{}{
			"name": pod.Name, "namespace": pod.Namespace, "uid": pod.UID,
		},
		"spec": map[string]interface{}{
			"serviceAccountName": pod.ServiceAccount,
			"hostNetwork":        pod.HostNetwork,
			"hostPID":            pod.HostPID,
			"hostIPC":            pod.HostIPC,
			"containers":         containers,
		},
	}
	rawBytes, err := json.Marshal(k8sPod)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"kind":       "Pod",
		"uid":        pod.UID,
		"name":       pod.Name,
		"namespace":  pod.Namespace,
		"cluster_id": pod.ClusterID,
		"raw_json":   string(rawBytes),
	}, nil
}

func resolveRulesDirectory() string {
	// Keep in sync with getRulesDirectory() in engine.go (unexported).
	if dir := strings.TrimSpace(os.Getenv("FORTUNA_RULES_DIR")); dir != "" {
		return dir
	}
	if _, err := os.Stat("./rules"); err == nil {
		return "./rules"
	}
	if _, err := os.Stat("core/rules"); err == nil {
		return "core/rules"
	}
	return ""
}

func envBool(name string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func envString(name, def string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	return v
}

func envInt(name string, def int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func envDuration(name string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return def
	}
	return d
}

func severityRank(sev string) int {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical", "sev1", "p0":
		return 4
	case "high", "sev2", "p1":
		return 3
	case "medium", "warning", "sev3", "p2":
		return 2
	case "low", "notice", "info", "informational", "debug", "sev4", "p3":
		return 1
	default:
		return 0
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
