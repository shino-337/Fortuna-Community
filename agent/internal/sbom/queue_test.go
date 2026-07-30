package sbom

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestEnqueue_DifferentUIDSameName ensures two pods with same namespace/name but different UIDs
// are both queued (Finding #2: key by UID so recycled pods get processed).
func TestEnqueue_DifferentUIDSameName(t *testing.T) {
	proc := &Processor{} // nil extractor/client ok for enqueue-only
	q := NewWorkQueue(proc, 2)
	q.Start()
	defer q.Stop()

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nginx", UID: "uid-111"},
		Spec:      corev1.PodSpec{NodeName: "node-1"},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "nginx", UID: "uid-222"},
		Spec:      corev1.PodSpec{NodeName: "node-1"},
	}

	ok1 := q.Enqueue(pod1)
	ok2 := q.Enqueue(pod2)
	if !ok1 {
		t.Fatal("expected first pod (uid-111) to be queued")
	}
	if !ok2 {
		t.Fatal("expected second pod (uid-222) same name to be queued (key is UID)")
	}

	// Drain queue so workers don't block on nil processor
	var received int
	for received < 2 {
		select {
		case <-q.Queue():
			received++
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for 2 pods, got %d", received)
		}
	}
}

// TestPodKey_usesUID verifies podKey returns UID.
func TestPodKey_usesUID(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "n", UID: "my-uid-123"},
	}
	if got := podKey(pod); got != "my-uid-123" {
		t.Errorf("podKey() = %q, want my-uid-123", got)
	}
	if got := podKey(nil); got != "" {
		t.Errorf("podKey(nil) = %q, want \"\"", got)
	}
}

// TestEnqueue_SameUIDIgnored ensures the same pod (same UID) enqueued twice is skipped the second time.
func TestEnqueue_SameUIDIgnored(t *testing.T) {
	proc := &Processor{}
	q := NewWorkQueue(proc, 1)
	// Do not start workers so the single pod stays in the channel and active stays set

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "x", UID: "same-uid"},
		Spec:      corev1.PodSpec{NodeName: "n"},
	}
	if !q.Enqueue(pod) {
		t.Fatal("first enqueue should succeed")
	}
	ok2 := q.Enqueue(pod)
	if ok2 {
		t.Error("second enqueue same UID should be skipped")
	}

	// Drain the one pod from the channel so we can Stop without leaving a worker blocked
	<-q.Queue()
	q.Stop()
}
