package watcher

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func TestPodFromDeleteObject(t *testing.T) {
	pod := &corev1.Pod{}
	pod.Name = "demo"
	pod.Namespace = "fortuna"
	pod.UID = types.UID("pod-uid")

	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{name: "direct pod", obj: pod, want: true},
		{name: "tombstone pod", obj: cache.DeletedFinalStateUnknown{Obj: pod}, want: true},
		{name: "unexpected object", obj: "not-a-pod", want: false},
		{name: "unexpected tombstone object", obj: cache.DeletedFinalStateUnknown{Obj: "not-a-pod"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := podFromDeleteObject(tt.obj)
			if ok != tt.want {
				t.Fatalf("ok=%v, want %v", ok, tt.want)
			}
			if ok && got != pod {
				t.Fatalf("got pod %#v, want original pod", got)
			}
		})
	}
}
