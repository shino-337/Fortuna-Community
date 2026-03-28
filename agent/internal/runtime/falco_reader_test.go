package runtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestFalcoReader_ToRuntimeEvent_MinimalMapping(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", 0, "http://core", "node-1", nil)
	fe := falcoEvent{
		Rule:     "Terminal shell in container",
		Priority: "Warning",
		Tags:     []string{"mitre_technique=T1059"},
		OutputFields: map[string]interface{}{
			"k8s.pod.uid":   "11111111-1111-1111-1111-111111111111",
			"k8s.ns.name":   "default",
			"k8s.pod.name":  "demo",
			"k8s.node.name": "node-1",
			"evt.type":      "execve",
			"proc.cmdline":  "/bin/sh -c id",
		},
	}
	ev, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.Runtime != "falco" || ev.Syscall != "execve" {
		t.Fatalf("unexpected ev: %+v", ev)
	}
	if ev.MitreTechnique != "T1059" {
		t.Fatalf("mitre: %q", ev.MitreTechnique)
	}
	if ev.Severity != "medium" {
		t.Fatalf("severity: %q", ev.Severity)
	}
	if ev.Pod["uid"] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("pod uid: %v", ev.Pod["uid"])
	}
	b, _ := json.Marshal(ev)
	if len(b) == 0 {
		t.Fatalf("marshal empty")
	}
}

func TestExtractMitreTechnique_FallbackPlainTag(t *testing.T) {
	got := extractMitreTechnique([]string{"something", "T1611"})
	if got != "T1611" {
		t.Fatalf("got %q", got)
	}
}

func TestParseFalcoJSONLines_singleAndConcatenated(t *testing.T) {
	t.Helper()
	one := []byte(`{"rule":"x","priority":"Notice","output_fields":{"k8s.pod.uid":"u"}}`)
	got, err := parseFalcoJSONLines(one)
	if err != nil || len(got) != 1 || got[0].Rule != "x" {
		t.Fatalf("single: got %+v err %v", got, err)
	}
	two := []byte(`{"rule":"a","priority":"Notice"}{"rule":"b","priority":"Notice"}`)
	got2, err2 := parseFalcoJSONLines(two)
	if err2 != nil || len(got2) != 2 || got2[0].Rule != "a" || got2[1].Rule != "b" {
		t.Fatalf("concat: got %+v err %v", got2, err2)
	}
}

func TestFalcoReader_ToRuntimeEvent_FallbackSyscallWhenEvtTypeMissing(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", time.Second, "http://core", "node-1", nil)
	fe := falcoEvent{
		Rule:     "Write below etc",
		Priority: "Notice",
		OutputFields: map[string]interface{}{
			"k8s.pod.uid":  "22222222-2222-2222-2222-222222222222",
			"k8s.ns.name":  "default",
			"k8s.pod.name": "demo",
			"fd.name":      "/etc/foo",
		},
	}
	ev, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.Syscall != "falco.alert" {
		t.Fatalf("syscall: %q", ev.Syscall)
	}
}

func TestFalcoReader_ToRuntimeEvent_PreservesResolutionStateFromFields(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", time.Second, "http://core", "node-1", nil)
	fe := falcoEvent{
		Rule: "Some Falco Rule",
		OutputFields: map[string]interface{}{
			"k8s.pod.uid":              "33333333-3333-3333-3333-333333333333",
			"k8s.ns.name":              "default",
			"k8s.pod.name":             "demo",
			"evt.type":                 "openat",
			"fortuna.resolution_state": "resolved",
		},
	}
	ev, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.ResolutionState != "resolved" {
		t.Fatalf("resolution_state: %q", ev.ResolutionState)
	}
}

func TestFalcoReader_ToRuntimeEvent_PreservesResolutionStateFromTags(t *testing.T) {
	r := NewFalcoReader("/tmp/falco.jsonl", time.Second, "http://core", "node-1", nil)
	fe := falcoEvent{
		Rule: "Some Falco Rule",
		Tags: []string{"runtime", "resolution_state=partial"},
		OutputFields: map[string]interface{}{
			"k8s.pod.uid":  "44444444-4444-4444-4444-444444444444",
			"k8s.ns.name":  "default",
			"k8s.pod.name": "demo",
			"evt.type":     "openat",
		},
	}
	ev, ok := r.toRuntimeEvent(context.Background(), &fe)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if ev.ResolutionState != "partial" {
		t.Fatalf("resolution_state: %q", ev.ResolutionState)
	}
}
