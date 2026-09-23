package collection

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const RuntimeProducerManifestVersion = 1
const RuntimeProducerLeaseMaxAge = time.Minute
const RuntimeProducerHeartbeatInterval = 20 * time.Second

const (
	RuntimeAgentRunning  = "running"
	RuntimeAgentStopping = "stopping"

	RuntimeProducerStarting         = "starting"
	RuntimeProducerActive           = "active"
	RuntimeProducerDegraded         = "degraded"
	RuntimeProducerDisabled         = "disabled"
	RuntimeProducerNonAuthoritative = "non_authoritative"
	RuntimeProducerStopped          = "stopped"
)

var runtimeProducerRegistry = map[string]string{
	"runtime-file": RuntimeSourceFile,
	"falco":        RuntimeSourceFalco,
	"ebpf-exec":    RuntimeSourceEBPF,
	"ebpf-connect": RuntimeSourceEBPF,
	"ebpf-all":     RuntimeSourceEBPF,
}

type RuntimeProducerDeclaration struct {
	ProducerID    string `json:"producerId"`
	SourceKind    string `json:"sourceKind"`
	Enabled       bool   `json:"enabled"`
	Authoritative bool   `json:"authoritative"`
}

type RuntimeProducerManifest struct {
	Version          int                          `json:"version"`
	SessionID        string                       `json:"sessionId"`
	SessionStartedAt time.Time                    `json:"sessionStartedAt"`
	ReportedAt       time.Time                    `json:"reportedAt"`
	AgentState       string                       `json:"agentState"`
	Producers        []RuntimeProducerDeclaration `json:"producers"`
}

func (m RuntimeProducerManifest) Validate(now time.Time) error {
	if m.Version != RuntimeProducerManifestVersion || len(m.SessionID) < 16 || len(m.SessionID) > 128 || strings.TrimSpace(m.SessionID) != m.SessionID {
		return fmt.Errorf("invalid runtime producer session")
	}
	if m.AgentState != RuntimeAgentRunning && m.AgentState != RuntimeAgentStopping {
		return fmt.Errorf("invalid runtime agent state")
	}
	if m.SessionStartedAt.IsZero() || m.ReportedAt.IsZero() || m.ReportedAt.Before(m.SessionStartedAt) ||
		m.ReportedAt.After(now.Add(time.Minute)) || m.ReportedAt.Before(now.Add(-2*RuntimeProducerLeaseMaxAge)) {
		return fmt.Errorf("invalid runtime producer manifest time")
	}
	if len(m.Producers) != len(runtimeProducerRegistry) {
		return fmt.Errorf("runtime producer manifest must declare the complete producer registry")
	}
	seen := map[string]bool{}
	for _, p := range m.Producers {
		expected, ok := runtimeProducerRegistry[p.ProducerID]
		if !ok || expected != p.SourceKind || seen[p.ProducerID] {
			return fmt.Errorf("invalid runtime producer declaration")
		}
		seen[p.ProducerID] = true
		// Manifest v1 has no independent upstream source-health proof. Therefore
		// no current producer may claim absence authority merely from Agent
		// configuration or reader liveness. A future protocol version may add a
		// verifiable health contract and explicitly relax this rule.
		if p.Authoritative {
			return fmt.Errorf("runtime producer manifest v1 does not accept authoritative producers")
		}
		if m.AgentState == RuntimeAgentStopping && p.Enabled {
			return fmt.Errorf("stopping runtime agent cannot declare enabled producers")
		}
	}
	return nil
}

func RuntimeProducerRegistrySize() int {
	return len(runtimeProducerRegistry)
}

func RuntimeProducerSource(producerID string) (string, bool) {
	source, ok := runtimeProducerRegistry[producerID]
	return source, ok
}

func SortRuntimeProducerDeclarations(in []RuntimeProducerDeclaration) {
	sort.Slice(in, func(i, j int) bool { return in[i].ProducerID < in[j].ProducerID })
}
