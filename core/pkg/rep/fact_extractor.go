package rep

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type behaviorFactDraft struct {
	FactType string
	Domain   string
	Attrs    map[string]interface{}
}

// extractAndPersistBehaviorFacts is P0 minimal REP-A: runtime_events -> runtime_behavior_facts.
// It is deterministic and idempotent per (event_id, fact_type).
func extractAndPersistBehaviorFacts(ctx context.Context, db *gorm.DB, event *models.RuntimeEvent) ([]models.RuntimeBehaviorFact, error) {
	if db == nil || event == nil || event.ID == 0 || strings.TrimSpace(event.PodUID) == "" {
		return nil, nil
	}

	facts := deriveBehaviorFacts(event)
	out := make([]models.RuntimeBehaviorFact, 0, len(facts))
	for _, f := range facts {
		attrsJSON, _ := json.Marshal(f.Attrs)
		sourceRefJSON, _ := json.Marshal(map[string]interface{}{
			"source_kind": event.Runtime,
			"source_rule": event.Signal,
			"event_id":    event.ID,
		})
		row := models.RuntimeBehaviorFact{
			ClusterID:  event.ClusterID,
			FactID:     fmt.Sprintf("%d:%s", event.ID, f.FactType),
			EventID:    event.ID,
			PodUID:     event.PodUID,
			Namespace:  event.Namespace,
			FactType:   f.FactType,
			Domain:     f.Domain,
			Attributes: string(attrsJSON),
			SourceRef:  string(sourceRefJSON),
			ObservedAt: event.CreatedAt,
			CreatedAt:  event.CreatedAt,
		}
		// idempotent upsert, safe for replays/retries
		if err := db.WithContext(ctx).
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "fact_id"}},
				DoNothing: true,
			}).
			Create(&row).Error; err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func deriveBehaviorFacts(event *models.RuntimeEvent) []behaviorFactDraft {
	syscall := strings.ToLower(strings.TrimSpace(event.Syscall))
	target := strings.TrimSpace(event.TargetPath)
	rt := strings.ToLower(strings.TrimSpace(event.Runtime))
	out := make([]behaviorFactDraft, 0, 4)

	add := func(factType, domain string, attrs map[string]interface{}) {
		out = append(out, behaviorFactDraft{
			FactType: factType,
			Domain:   domain,
			Attrs:    attrs,
		})
	}

	// execution family
	if syscall == "execve" || syscall == "exec" {
		lower := strings.ToLower(target)
		capability := strings.ToUpper(strings.TrimSpace(event.Capability))
		add("PROCESS_EXEC", "execution", map[string]interface{}{
			"syscall": syscall,
			"target":  target,
			"runtime": rt,
			"capability": capability,
		})
		// Keep eBPF exec traces mapped to EBPF_EXEC_ACTIVITY in legacy REP path,
		// avoid overriding with INTERACTIVE_SHELL_EXEC from fact synthesis.
		if capability != "EBPF_EXEC_TRACE" &&
			(strings.Contains(lower, "sh") || strings.Contains(lower, "bash") || strings.Contains(lower, "zsh")) {
			add("INTERACTIVE_SHELL", "execution", map[string]interface{}{
				"target": target,
			})
		}
		if strings.HasPrefix(lower, "/tmp/") || strings.HasPrefix(lower, "/dev/shm/") {
			add("TMP_BINARY_EXEC", "execution", map[string]interface{}{
				"target": target,
			})
		}
		if strings.Contains(lower, "curl") || strings.Contains(lower, "wget") {
			add("REMOTE_TOOL_EXEC", "execution", map[string]interface{}{
				"target": target,
			})
		}
	}

	// filesystem family
	if syscall == "open" || syscall == "openat" || syscall == "read" || syscall == "stat" || syscall == "readlink" {
		add("FILE_READ", "filesystem", map[string]interface{}{
			"path":    target,
			"syscall": syscall,
		})
		if strings.Contains(target, "/var/run/secrets/kubernetes.io/serviceaccount/token") {
			add("SERVICEACCOUNT_TOKEN_READ", "credentials", map[string]interface{}{
				"path": target,
			})
		}
		if strings.Contains(target, "/run/containerd/containerd.sock") || strings.Contains(target, "/var/run/docker.sock") {
			add("HOST_PATH_TOUCH", "filesystem", map[string]interface{}{
				"path": target,
			})
		}
	}

	// network family
	if syscall == "connect" {
		add("NETWORK_CONNECT", "network", map[string]interface{}{
			"target": target,
		})
		t := strings.ToLower(target)
		if strings.Contains(t, "dst=") || strings.Contains(t, "dport=") || strings.Contains(t, ":") {
			add("EXTERNAL_CONNECT", "network", map[string]interface{}{
				"target": target,
			})
		}
	}

	return out
}
