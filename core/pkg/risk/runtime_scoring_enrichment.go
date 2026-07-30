package risk

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/fortuna/core/pkg/k8scorroboration"
	"github.com/fortuna/core/pkg/models"
)

// Capability IDs must match github.com/fortuna/core/pkg/capability constants;
// literals avoid an import cycle (capability → risk).
const (
	runtimeDerivedEscRuntimeActive   = "ESC_RUNTIME_ACTIVE"
	runtimeDerivedEscRuntimeProcRoot = "ESC_RUNTIME_PROC_ROOT"
	runtimeDerivedEscHostpathNode    = "ESC_HOSTPATH_NODE"
	runtimeDerivedEscPrivPod         = "ESC_PRIV_POD"
	runtimeDerivedIDTokenPod         = "ID_TOKEN_POD"
)

func runtimeSignalTypeSet(signals []models.RuntimeSignal) map[string]bool {
	out := make(map[string]bool)
	for _, rs := range signals {
		st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
		if st != "" {
			out[st] = true
		}
	}
	return out
}

// hasRuntimeInteractiveShellSignals reports exec/shell-class runtime signals.
func hasRuntimeInteractiveShellSignals(signals []models.RuntimeSignal) bool {
	t := runtimeSignalTypeSet(signals)
	return t["INTERACTIVE_SHELL_EXEC"] || t["SUSPICIOUS_EXEC_FROM_SNAPSHOT"] || t["EBPF_EXEC_ACTIVITY"]
}

func effectiveSignalConfidence(rs models.RuntimeSignal) float64 {
	c := rs.Confidence
	if c <= 0 {
		return 0.5
	}
	if c > 1 {
		return 1
	}
	return c
}

func maxConfidenceForSignalTypes(signals []models.RuntimeSignal, types ...string) float64 {
	want := make(map[string]bool)
	for _, t := range types {
		want[strings.ToUpper(strings.TrimSpace(t))] = true
	}
	var m float64
	for _, rs := range signals {
		st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
		if !want[st] {
			continue
		}
		c := effectiveSignalConfidence(rs)
		if c > m {
			m = c
		}
	}
	if m == 0 {
		return 0.5
	}
	return m
}

// exploitStateConfidenceThreshold is the minimum signal confidence required to
// persist an "exploited" state for a given injected capability (token abuse is stricter).
func exploitStateConfidenceThreshold(capID string) float64 {
	switch strings.ToUpper(strings.TrimSpace(capID)) {
	case runtimeDerivedIDTokenPod:
		return 0.7
	case runtimeDerivedEscRuntimeActive:
		return 0.5
	default:
		return 0.6
	}
}

func capRuntimeStateByConfidenceForCapability(desired string, conf float64, capID string) string {
	thr := exploitStateConfidenceThreshold(capID)
	if conf < thr && stateRank(desired) >= stateRank("exploited") {
		return "confirmed"
	}
	return desired
}

func stateRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "exploited":
		return 4
	case "chained":
		return 3
	case "confirmed":
		return 2
	case "detected":
		return 1
	default:
		return 0
	}
}

func severityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func strongerCapabilityRow(cur, next models.PodCapability) models.PodCapability {
	if stateRank(next.State) > stateRank(cur.State) {
		return next
	}
	if stateRank(next.State) == stateRank(cur.State) && severityRank(next.Severity) > severityRank(cur.Severity) {
		return next
	}
	return cur
}

type registryCapAgg struct {
	maxConf       float64
	state         string
	severity      string
	sourceSignals map[string]struct{}
}

type runtimeCapabilityProvenance struct {
	Source       string              `json:"source"`
	SignalTypes  []string            `json:"source_signal_types,omitempty"`
	SourceEvents []provenanceEventRef `json:"source_events,omitempty"`
}

type provenanceEventRef struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref,omitempty"`
	Signal  string `json:"signal_type,omitempty"`
	EventID string `json:"event_id,omitempty"`
}

func k8sCorroboratingRuntimeEventRefs(events []models.RuntimeEvent) []string {
	var out []string
	for i := range events {
		e := &events[i]
		if !k8scorroboration.RuntimeEventCorroboratesK8sAPI(e) {
			continue
		}
		ref := strings.TrimSpace(e.EventID)
		if ref == "" {
			ref = "runtime_event:" + strconv.FormatUint(uint64(e.ID), 10)
		}
		out = append(out, ref)
	}
	return out
}

func runtimeDerivedProvenanceJSON(agg *registryCapAgg, capID string, k8sEventRefs []string) string {
	if agg == nil {
		return ""
	}
	types := make([]string, 0, len(agg.sourceSignals))
	for st := range agg.sourceSignals {
		types = append(types, st)
	}
	sort.Strings(types)
	p := runtimeCapabilityProvenance{Source: "runtime", SignalTypes: types}
	idTok := strings.EqualFold(strings.TrimSpace(capID), runtimeDerivedIDTokenPod)
	if idTok && len(k8sEventRefs) > 0 {
		for _, ref := range k8sEventRefs {
			p.SourceEvents = append(p.SourceEvents, provenanceEventRef{Kind: "k8s_api_corroboration", Ref: ref})
		}
	}
	for _, st := range types {
		p.SourceEvents = append(p.SourceEvents, provenanceEventRef{Kind: "runtime_signal", Signal: st})
	}
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func applyBindingExploitFloor(desired string, conf float64, b SignalCapabilityBinding) string {
	if strings.ToLower(strings.TrimSpace(desired)) != "exploited" {
		return desired
	}
	if b.ExploitConfFloor <= 0 {
		return desired
	}
	if conf < b.ExploitConfFloor {
		return "confirmed"
	}
	return desired
}

func registryInjectionSeverity(signalType string, b SignalCapabilityBinding) string {
	_ = b
	if strings.ToUpper(strings.TrimSpace(signalType)) == "NAMESPACE_ESCAPE" {
		return "critical"
	}
	return "high"
}

// aggregateRegistryInjections merges per CapabilityID so inject stays idempotent (spec III, XIII).
func aggregateRegistryInjections(signals []models.RuntimeSignal, events []models.RuntimeEvent) map[string]*registryCapAgg {
	hasShell := hasRuntimeInteractiveShellSignals(signals)
	k8sOK := CorroboratesK8sAPIAccess(events, signals)
	toxic := hasShell && k8sOK

	out := make(map[string]*registryCapAgg)
	for _, rs := range signals {
		st := strings.ToUpper(strings.TrimSpace(rs.SignalType))
		if st == "" {
			continue
		}
		b := ResolveSignalCapability(st)
		conf := effectiveSignalConfidence(rs)
		desired := b.BaseState
		if st == "NETWORK_QUEUE_ANOMALY" {
			desired = "confirmed"
			if toxic {
				desired = "exploited"
			}
		}
		desired = applyBindingExploitFloor(desired, conf, b)
		sev := registryInjectionSeverity(st, b)
		cid := strings.ToUpper(strings.TrimSpace(b.CapabilityID))
		cur, ok := out[cid]
		if !ok {
			out[cid] = &registryCapAgg{
				maxConf: conf, state: desired, severity: sev,
				sourceSignals: map[string]struct{}{st: {}},
			}
			continue
		}
		if cur.sourceSignals == nil {
			cur.sourceSignals = map[string]struct{}{}
		}
		cur.sourceSignals[st] = struct{}{}
		if conf > cur.maxConf {
			cur.maxConf = conf
		}
		if stateRank(desired) > stateRank(cur.state) {
			cur.state = desired
		}
		if severityRank(sev) > severityRank(cur.severity) {
			cur.severity = sev
		}
	}
	return out
}

// mergeRuntimeDerivedPodCapabilities overlays scoring-only capability rows from
// high-signal runtime detections so active exploitation lifts capability exposure
// (and blast radius) without requiring a full graph rewrite.
//
// Runtime-derived capabilities are an explicit "runtime overrides static model"
// channel: when telemetry proves a step (e.g. API contact with SA token path),
// we surface ID_TOKEN_POD / escape caps for scoring even if the graph edge was latent.
//
// Idempotency: at most one runtime_derived row per CapabilityID per scoring pass;
// tryInject coalesces via byID and strongerCapabilityRow.
func mergeRuntimeDerivedPodCapabilities(podUID, namespace string, base []models.PodCapability, signals []models.RuntimeSignal, events []models.RuntimeEvent) ([]models.PodCapability, []string) {
	if len(signals) == 0 {
		return base, nil
	}

	out := make([]models.PodCapability, len(base))
	copy(out, base)
	byID := make(map[string]int)
	for i := range out {
		id := strings.ToUpper(strings.TrimSpace(out[i].CapabilityID))
		if id != "" {
			byID[id] = i
		}
	}

	var injected []string
	k8sEvtRefs := k8sCorroboratingRuntimeEventRefs(events)

	tryInject := func(capID, severity, state string, injectConf float64, agg *registryCapAgg) {
		id := strings.ToUpper(strings.TrimSpace(capID))
		if id == "" {
			return
		}
		state = capRuntimeStateByConfidenceForCapability(state, injectConf, id)
		if injectConf <= 0 {
			injectConf = 0.5
		}
		if injectConf > 1 {
			injectConf = 1
		}
		prov := runtimeDerivedProvenanceJSON(agg, id, k8sEvtRefs)
		row := models.PodCapability{
			PodUID:          podUID,
			Namespace:       namespace,
			CapabilityID:    id,
			CapabilityGroup: "runtime_derived",
			Severity:        severity,
			State:           state,
			Confidence:      injectConf,
			DerivedFrom:     prov,
		}
		if idx, ok := byID[id]; ok {
			merged := strongerCapabilityRow(out[idx], row)
			if merged.State != out[idx].State || merged.Severity != out[idx].Severity {
				out[idx] = merged
				injected = append(injected, id)
			}
			return
		}
		out = append(out, row)
		byID[id] = len(out) - 1
		injected = append(injected, id)
	}

	for cid, a := range aggregateRegistryInjections(signals, events) {
		tryInject(cid, a.severity, a.state, a.maxConf, a)
	}

	seenInj := map[string]bool{}
	uniqInj := make([]string, 0, len(injected))
	for _, id := range injected {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == "" || seenInj[id] {
			continue
		}
		seenInj[id] = true
		uniqInj = append(uniqInj, id)
	}
	return out, uniqInj
}
