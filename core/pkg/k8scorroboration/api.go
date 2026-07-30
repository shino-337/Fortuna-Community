// Package k8scorroboration implements K8s API reachability corroboration for runtime telemetry.
// It is shared by graph chain enrichment and risk scoring to avoid import cycles (graph ↔ risk).
package k8scorroboration

import (
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
)

// DefaultSATokenSatisfactionHalfLife controls decay of SA_TOKEN graph satisfaction from K8s API corroboration age.
const DefaultSATokenSatisfactionHalfLife = 6 * time.Hour

// RuntimeSatisfiedGraphRequirements lists graph precondition capability IDs that runtime
// telemetry has effectively closed (e.g. SA_TOKEN when K8s API access is corroborated).
func RuntimeSatisfiedGraphRequirements(events []models.RuntimeEvent, signals []models.RuntimeSignal) []string {
	if CorroboratesK8sAPIAccess(events, signals) {
		return []string{"SA_TOKEN"}
	}
	return []string{}
}

// CorroboratesK8sAPIAccess returns true only when there is evidence that outbound
// traffic targets the Kubernetes API (not generic NETWORK_QUEUE_ANOMALY noise).
func CorroboratesK8sAPIAccess(events []models.RuntimeEvent, signals []models.RuntimeSignal) bool {
	for i := range events {
		if RuntimeEventCorroboratesK8sAPI(&events[i]) {
			return true
		}
	}
	for i := range signals {
		if NetworkQueueSignalEvidenceIndicatesK8sAPI(&signals[i]) {
			return true
		}
	}
	return false
}

// RuntimeEventCorroboratesK8sAPI is exported for temporal / ordering logic in risk package.
func RuntimeEventCorroboratesK8sAPI(e *models.RuntimeEvent) bool {
	return runtimeEventCorroboratesK8sAPI(e)
}

// NetworkQueueSignalEvidenceIndicatesK8sAPI is exported for temporal / ordering logic in risk package.
func NetworkQueueSignalEvidenceIndicatesK8sAPI(rs *models.RuntimeSignal) bool {
	return networkQueueSignalEvidenceIndicatesK8sAPI(rs)
}

func runtimeEventStrongK8sAPISourceRule(e *models.RuntimeEvent) bool {
	if e == nil {
		return false
	}
	rule := strings.ToLower(strings.TrimSpace(e.SourceRule))
	if rule == "" {
		return false
	}
	if strings.Contains(rule, "k8s api") || strings.Contains(rule, "kubernetes api") {
		return true
	}
	if strings.Contains(rule, "contact") && strings.Contains(rule, "k8s") {
		return true
	}
	if strings.Contains(rule, "apiserver") || strings.Contains(rule, "api server") {
		return true
	}
	return false
}

func isLikelyNonK8sAPIEvent(e *models.RuntimeEvent) bool {
	if e == nil {
		return false
	}
	tgt := strings.ToLower(strings.TrimSpace(e.TargetPath))
	if tgt != "" {
		if strings.Contains(tgt, "dns") || strings.Contains(tgt, "kube-dns") || strings.Contains(tgt, "coredns") {
			return true
		}
		if strings.Contains(tgt, "health") || strings.Contains(tgt, "ready") || strings.Contains(tgt, "live") {
			return true
		}
	}
	if payloadIndicatesUDPProtocol(e.PayloadJSON) {
		return true
	}
	return false
}

func runtimeEventCorroboratesK8sAPI(e *models.RuntimeEvent) bool {
	if e == nil {
		return false
	}
	if runtimeEventStrongK8sAPISourceRule(e) {
		return true
	}
	if isLikelyNonK8sAPIEvent(e) {
		return false
	}
	tgt := strings.ToLower(strings.TrimSpace(e.TargetPath))
	if strings.Contains(tgt, "kubernetes.default") {
		return true
	}
	return payloadJSONIndicatesK8sAPI(e.PayloadJSON)
}

func networkQueueSignalEvidenceIndicatesK8sAPI(rs *models.RuntimeSignal) bool {
	if rs == nil {
		return false
	}
	if strings.ToUpper(strings.TrimSpace(rs.SignalType)) != "NETWORK_QUEUE_ANOMALY" {
		return false
	}
	if signalEvidenceLikelyNonK8s(rs) {
		return false
	}
	return jsonBlobIndicatesK8sAPI(rs.Evidence) || jsonBlobIndicatesK8sAPI(rs.EvidenceRefs)
}

func signalEvidenceLikelyNonK8s(rs *models.RuntimeSignal) bool {
	ev := strings.ToLower(rs.Evidence + " " + rs.EvidenceRefs)
	if ev == "" {
		return false
	}
	if strings.Contains(ev, ":53") || strings.Contains(ev, `"port":53`) || strings.Contains(ev, `"port":"53"`) {
		return true
	}
	if strings.Contains(ev, "kube-dns") || strings.Contains(ev, "coredns") || strings.Contains(ev, "/dns") {
		return true
	}
	if strings.Contains(ev, "health") && !strings.Contains(ev, "kubernetes.default") {
		return true
	}
	return false
}

func payloadIndicatesUDPProtocol(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return false
	}
	low := strings.ToLower(raw)
	if strings.Contains(low, `"protocol":"udp"`) || strings.Contains(low, `"protocol": "udp"`) {
		return true
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return false
	}
	if s, ok := stringish(m["protocol"]); ok && strings.EqualFold(strings.TrimSpace(s), "udp") {
		return true
	}
	if meta, ok := m["meta"].(map[string]interface{}); ok {
		if s, ok := stringish(meta["protocol"]); ok && strings.EqualFold(strings.TrimSpace(s), "udp") {
			return true
		}
	}
	return mapContainsUDPProtocol(m)
}

func mapContainsUDPProtocol(m map[string]interface{}) bool {
	if s, ok := stringish(m["protocol"]); ok && strings.EqualFold(strings.TrimSpace(s), "udp") {
		return true
	}
	for _, v := range m {
		switch t := v.(type) {
		case map[string]interface{}:
			if mapContainsUDPProtocol(t) {
				return true
			}
		}
	}
	return false
}

func payloadJSONIndicatesK8sAPI(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return false
	}
	if jsonBlobIndicatesK8sAPI(raw) {
		return true
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return false
	}
	return mapContainsK8sAPIMarker(m)
}

func jsonBlobIndicatesK8sAPI(s string) bool {
	low := strings.ToLower(s)
	if strings.Contains(low, "kubernetes.default") {
		return true
	}
	if strings.Contains(low, `"k8s_api":true`) || strings.Contains(low, `"k8s_api":"true"`) {
		return true
	}
	if strings.Contains(low, "k8s_api") && strings.Contains(low, "true") {
		return true
	}
	if strings.Contains(low, "fd.name") && strings.Contains(low, "kubernetes") {
		return true
	}
	return false
}

func mapContainsK8sAPIMarker(m map[string]interface{}) bool {
	if truthyMeta(m["k8s_api"]) || truthyMeta(m["k8sApi"]) {
		return true
	}
	if s, ok := stringish(m["dst_ip"]); ok {
		if strings.EqualFold(strings.TrimSpace(s), "kubernetes.default") {
			return true
		}
	}
	if s, ok := stringish(m["destination"]); ok && jsonBlobIndicatesK8sAPI(s) {
		return true
	}
	for _, v := range m {
		switch t := v.(type) {
		case map[string]interface{}:
			if mapContainsK8sAPIMarker(t) {
				return true
			}
		case string:
			if jsonBlobIndicatesK8sAPI(t) {
				return true
			}
		}
	}
	return false
}

func truthyMeta(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true") || strings.TrimSpace(t) == "1"
	case float64:
		return t != 0
	default:
		return false
	}
}

func stringish(v interface{}) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	default:
		return "", false
	}
}

// SATokenSatisfactionWeight is a time-decayed [0,1] weight for treating SA_TOKEN as runtime-satisfied on the graph.
// Uses the freshest corroborating evidence timestamp and the strongest contributing event confidence.
func SATokenSatisfactionWeight(events []models.RuntimeEvent, signals []models.RuntimeSignal, now time.Time, halfLife time.Duration) float64 {
	t, conf, ok := latestK8sAPICorroborationMeta(events, signals)
	if !ok {
		return 0
	}
	if halfLife <= 0 {
		halfLife = DefaultSATokenSatisfactionHalfLife
	}
	age := now.Sub(t)
	if age < 0 {
		age = 0
	}
	decay := math.Exp(-float64(age) / float64(halfLife))
	w := decay * conf
	if w > 1 {
		return 1
	}
	if w < 0 {
		return 0
	}
	return w
}

func latestK8sAPICorroborationMeta(events []models.RuntimeEvent, signals []models.RuntimeSignal) (latest time.Time, maxConf float64, ok bool) {
	for i := range events {
		e := &events[i]
		if !runtimeEventCorroboratesK8sAPI(e) {
			continue
		}
		t := e.CreatedAt
		if e.ObservedAt != nil && !e.ObservedAt.IsZero() {
			t = *e.ObservedAt
		}
		if !ok || t.After(latest) {
			latest = t
		}
		c := e.Confidence
		if c > maxConf {
			maxConf = c
		}
		ok = true
	}
	for i := range signals {
		rs := &signals[i]
		if !networkQueueSignalEvidenceIndicatesK8sAPI(rs) {
			continue
		}
		t := rs.CreatedAt
		if !ok || t.After(latest) {
			latest = t
		}
		c := rs.Confidence
		if c <= 0 {
			c = 0.5
		}
		if c > maxConf {
			maxConf = c
		}
		ok = true
	}
	if ok && maxConf <= 0 {
		maxConf = 0.75
	}
	return latest, maxConf, ok
}
