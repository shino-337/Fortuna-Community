package poddetail

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildContainerIDToPodMap(t *testing.T) {
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{UID: "pod-uid-1", Namespace: "ns1"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "app", ContainerID: "containerd://abc123def456"},
					{Name: "sidecar", ContainerID: "containerd://xyz789"},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{UID: "pod-uid-2", Namespace: "ns2"},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{Name: "main", ContainerID: "cri-o://111222333444"},
				},
			},
		},
	}
	m := BuildContainerIDToPodMap(pods)
	if len(m) == 0 {
		t.Fatal("expected non-empty map")
	}
	// containerd 12-char suffix from "abc123def456"
	if info, ok := m["abc123def456"]; ok {
		if info.PodUID != "pod-uid-1" || info.Namespace != "ns1" || info.ContainerName != "app" {
			t.Errorf("wrong info: %+v", info)
		}
	} else {
		t.Errorf("expected key abc123def456 in map: %v", m)
	}
	// xyz789 is 6 chars, short id is full
	if info, ok := m["xyz789"]; ok {
		if info.ContainerName != "sidecar" {
			t.Errorf("wrong container name: %s", info.ContainerName)
		}
	}
	// cri-o prefix trimmed
	if info, ok := m["111222333444"]; ok {
		if info.PodUID != "pod-uid-2" || info.ContainerName != "main" {
			t.Errorf("wrong info: %+v", info)
		}
	}
}

func TestNormalizeContainerIDShort(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"containerd://abc123def456", "abc123def456"},
		{"containerd://a1b2c3d4e5f6a1b2c3d4e5f6", "a1b2c3d4e5f6"}, // last 12 of 24-char id
		{"cri-o://xyz", "xyz"},
	}
	for _, tt := range tests {
		got := normalizeContainerIDShort(tt.in)
		if got != tt.want {
			t.Errorf("normalizeContainerIDShort(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTrimContainerIDPrefix(t *testing.T) {
	if got := trimContainerIDPrefix("containerd://id1"); got != "id1" {
		t.Errorf("trim = %q", got)
	}
	if got := trimContainerIDPrefix("cri-o://id2"); got != "id2" {
		t.Errorf("trim = %q", got)
	}
	if got := trimContainerIDPrefix("docker://id3"); got != "id3" {
		t.Errorf("trim = %q", got)
	}
	if got := trimContainerIDPrefix("id4"); got != "id4" {
		t.Errorf("trim = %q", got)
	}
}
