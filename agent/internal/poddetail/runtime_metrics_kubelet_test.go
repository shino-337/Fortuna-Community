package poddetail

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestBuildPodRuntimeUsageMap(t *testing.T) {
	raw := []byte(`{
	  "pods": [
	    {
	      "podRef": { "uid": "uid-a" },
	      "containers": [
	        {
	          "name": "c1",
	          "cpu": { "usageNanoCores": 210000000 },
	          "memory": { "workingSetBytes": 12345 }
	        },
	        {
	          "name": "c2",
	          "cpu": { "usageNanoCores": 1000000 },
	          "memory": { "workingSetBytes": 45678 }
	        }
	      ]
	    }
	  ]
	}`)
	got, err := buildPodRuntimeUsageMap(raw)
	if err != nil {
		t.Fatalf("buildPodRuntimeUsageMap error: %v", err)
	}
	if got["uid-a"]["c1"].CPUUsageMillicore != 210 {
		t.Fatalf("unexpected millicore for c1: %d", got["uid-a"]["c1"].CPUUsageMillicore)
	}
	if got["uid-a"]["c2"].CPUUsageMillicore != 1 {
		t.Fatalf("unexpected millicore for c2: %d", got["uid-a"]["c2"].CPUUsageMillicore)
	}
	if got["uid-a"]["c2"].MemoryUsageBytes != 45678 {
		t.Fatalf("unexpected memory bytes for c2: %d", got["uid-a"]["c2"].MemoryUsageBytes)
	}
}

func TestMemoryLimitBytesForContainer(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}
	got := memoryLimitBytesForContainer(pod, "app")
	const want int64 = 268435456
	if got != want {
		t.Fatalf("unexpected memory limit bytes: got=%d want=%d", got, want)
	}
	if memoryLimitBytesForContainer(pod, "missing") != 0 {
		t.Fatalf("expected zero for missing container")
	}
}
