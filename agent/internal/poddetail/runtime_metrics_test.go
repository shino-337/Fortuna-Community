package poddetail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestMergePodMetrics(t *testing.T) {
	raw := []byte(`{
	  "kind": "PodMetricsList",
	  "items": [
	    {
	      "metadata": { "name": "web", "namespace": "shop" },
	      "containers": [
	        { "name": "c1", "usage": { "cpu": "210000000n", "memory": "12345" } },
	        { "name": "c2", "usage": { "cpu": "1m", "memory": "64Ki" } }
	      ]
	    },
	    {
	      "metadata": { "name": "elsewhere", "namespace": "shop" },
	      "containers": [ { "name": "x", "usage": { "cpu": "5", "memory": "1Gi" } } ]
	    }
	  ]
	}`)
	got := make(podRuntimeUsageMap)
	if err := mergePodMetrics(got, raw, map[string]string{"shop/web": "uid-a"}); err != nil {
		t.Fatalf("mergePodMetrics error: %v", err)
	}
	if got["uid-a"]["c1"].CPUUsageMillicore != 210 {
		t.Fatalf("unexpected millicore for c1: %d", got["uid-a"]["c1"].CPUUsageMillicore)
	}
	if got["uid-a"]["c2"].CPUUsageMillicore != 1 {
		t.Fatalf("unexpected millicore for c2: %d", got["uid-a"]["c2"].CPUUsageMillicore)
	}
	if got["uid-a"]["c2"].MemoryUsageBytes != 65536 {
		t.Fatalf("unexpected memory bytes for c2: %d", got["uid-a"]["c2"].MemoryUsageBytes)
	}
	if len(got) != 1 {
		t.Fatalf("pods from other nodes must be ignored, got %v", got)
	}
}

func TestCollectPodRuntimeUsageNeverUsesNodeProxy(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"web","namespace":"shop"},"containers":[{"name":"c1","usage":{"cpu":"20m","memory":"1Mi"}}]}]}`))
	}))
	defer srv.Close()
	client, err := kubernetes.NewForConfig(&rest.Config{Host: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	pods := []corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "shop", UID: "uid-a"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "shop", UID: "uid-b"}},
	}
	got, err := collectPodRuntimeUsage(context.Background(), client, pods)
	if err != nil {
		t.Fatalf("collectPodRuntimeUsage error: %v", err)
	}
	if want := []string{"/apis/metrics.k8s.io/v1beta1/namespaces/shop/pods"}; strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected requests: got %v want %v", paths, want)
	}
	if got["uid-a"]["c1"].CPUUsageMillicore != 20 {
		t.Fatalf("unexpected usage: %v", got)
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
