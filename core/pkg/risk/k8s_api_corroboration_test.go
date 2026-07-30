package risk

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestCorroboratesK8sAPIAccess_sourceRule(t *testing.T) {
	ev := []models.RuntimeEvent{{SourceRule: "Contact K8s API Server From Container"}}
	if !CorroboratesK8sAPIAccess(ev, nil) {
		t.Fatal("expected true from SourceRule")
	}
}

func TestCorroboratesK8sAPIAccess_payloadMeta(t *testing.T) {
	ev := []models.RuntimeEvent{{PayloadJSON: `{"meta":{"k8s_api":true}}`}}
	if !CorroboratesK8sAPIAccess(ev, nil) {
		t.Fatal("expected true from payload k8s_api")
	}
}

func TestCorroboratesK8sAPIAccess_networkSignalEvidence(t *testing.T) {
	sig := []models.RuntimeSignal{{
		SignalType: "NETWORK_QUEUE_ANOMALY",
		Evidence:   `{"target":"https://kubernetes.default.svc:443"}`,
	}}
	if !CorroboratesK8sAPIAccess(nil, sig) {
		t.Fatal("expected true from evidence target")
	}
}

func TestCorroboratesK8sAPIAccess_networkOnlySignalRejected(t *testing.T) {
	sig := []models.RuntimeSignal{{
		SignalType: "NETWORK_QUEUE_ANOMALY",
		Evidence:   `{"target":"8.8.8.8:53"}`,
	}}
	if CorroboratesK8sAPIAccess(nil, sig) {
		t.Fatal("generic network evidence must not corroborate K8s API")
	}
}

func TestCorroboratesK8sAPIAccess_dnsTargetSuppressesWeakKubernetesHint(t *testing.T) {
	ev := []models.RuntimeEvent{{
		TargetPath:  "kube-dns.kube-system.svc.cluster.local:53",
		PayloadJSON: `{"fd.name":"kubernetes.default.svc"}`,
	}}
	if CorroboratesK8sAPIAccess(ev, nil) {
		t.Fatal("DNS-style target must suppress payload-only kubernetes hints without strong rule")
	}
}

func TestCorroboratesK8sAPIAccess_strongRuleBypassesNegativeHeuristics(t *testing.T) {
	ev := []models.RuntimeEvent{{
		SourceRule: "Contact K8s API Server From Container",
		TargetPath: "health.local:8080",
	}}
	if !CorroboratesK8sAPIAccess(ev, nil) {
		t.Fatal("named K8s API rule should corroborate regardless of health-like target")
	}
}

func TestCorroboratesK8sAPIAccess_udpPayloadRejected(t *testing.T) {
	ev := []models.RuntimeEvent{{
		PayloadJSON: `{"meta":{"protocol":"udp","k8s_api":true}}`,
	}}
	if CorroboratesK8sAPIAccess(ev, nil) {
		t.Fatal("UDP protocol hint should block weak corroboration")
	}
}
