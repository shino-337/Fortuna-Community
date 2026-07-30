package rep

import (
	"testing"
)

func TestClassifyFalcoRuleToSignal_RuleSubstrings(t *testing.T) {
	cases := []struct {
		rule     string
		syscall  string
		wantType string
		minConf  float64
	}{
		{"Terminal shell in container", "falco.alert", "SUSPICIOUS_EXEC_FROM_SNAPSHOT", 0.8},
		{"Launch Privileged Container", "falco.alert", "CAPABILITY_MISUSE", 0.75},
		{"Unexpected outbound connection destination", "falco.alert", "NETWORK_QUEUE_ANOMALY", 0.7},
		{"Read sensitive file untrusted", "falco.alert", "PROC_ROOT_PIVOT", 0.8},
		{"Mount was executed inside a container", "falco.alert", "FS_ESCAPE_ATTEMPT", 0.85},
		{"Change thread namespace", "falco.alert", "NAMESPACE_ESCAPE", 0.85},
	}
	for _, tc := range cases {
		sig, ok := ClassifyFalcoRuleToSignal(tc.rule, tc.syscall)
		if !ok {
			t.Fatalf("rule=%q: expected ok", tc.rule)
		}
		if sig.SignalType != tc.wantType {
			t.Fatalf("rule=%q: want signal %s, got %s", tc.rule, tc.wantType, sig.SignalType)
		}
		if sig.Confidence < tc.minConf {
			t.Fatalf("rule=%q: want confidence >= %.2f, got %.2f", tc.rule, tc.minConf, sig.Confidence)
		}
	}
}

func TestClassifyFalcoRuleToSignal_GenericAlert(t *testing.T) {
	sig, ok := ClassifyFalcoRuleToSignal("", "falco.alert")
	if !ok || sig.SignalType != "SUSPICIOUS_EXEC_FROM_SNAPSHOT" {
		t.Fatalf("generic falco.alert: got %+v ok=%v", sig, ok)
	}
}

func TestClassifyFalcoRuleToSignal_UnmappedNamedRule(t *testing.T) {
	sig, ok := ClassifyFalcoRuleToSignal("Custom vendor rule XYZ", "falco.alert")
	if !ok {
		t.Fatal("expected default bucket for named rule")
	}
	if sig.SignalType != "SUSPICIOUS_EXEC_FROM_SNAPSHOT" || sig.BaseScore < 40 {
		t.Fatalf("unexpected %+v", sig)
	}
}

func TestClassifyFalcoRuleToSignal_NonFalcoSyscallNoRule(t *testing.T) {
	_, ok := ClassifyFalcoRuleToSignal("", "execve")
	if ok {
		t.Fatal("execve without rule should not use generic falco.alert path")
	}
}
