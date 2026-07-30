package syncer

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/fortuna/agent/internal/cluster"
)

func TestOwnerRefsFromPod(t *testing.T) {
	t.Run("no_owner", func(t *testing.T) {
		p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{OwnerReferences: nil}}
		kind, name, rs := ownerRefsFromPod(p)
		if kind != "" || name != "" || rs != "" {
			t.Errorf("expected empty, got kind=%q name=%q replicaSetName=%q", kind, name, rs)
		}
	})

	t.Run("controller_is_deployment", func(t *testing.T) {
		ctrl := true
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "ReplicaSet", Name: "nginx-7d4f8b", Controller: &ctrl},
				},
			},
		}
		kind, name, rs := ownerRefsFromPod(p)
		if kind != "ReplicaSet" || name != "nginx-7d4f8b" || rs != "nginx-7d4f8b" {
			t.Errorf("got kind=%q name=%q replicaSetName=%q", kind, name, rs)
		}
	})

	t.Run("controller_is_statefulset", func(t *testing.T) {
		ctrl := true
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "StatefulSet", Name: "web", Controller: &ctrl},
				},
			},
		}
		kind, name, rs := ownerRefsFromPod(p)
		if kind != "StatefulSet" || name != "web" || rs != "" {
			t.Errorf("got kind=%q name=%q replicaSetName=%q", kind, name, rs)
		}
	})

	t.Run("non_controller_ignored", func(t *testing.T) {
		ctrl := false
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "ReplicaSet", Name: "rs", Controller: &ctrl},
				},
			},
		}
		kind, name, rs := ownerRefsFromPod(p)
		if kind != "" || name != "" || rs != "" {
			t.Errorf("expected empty (non-controller), got kind=%q name=%q replicaSetName=%q", kind, name, rs)
		}
	})
}

func TestQosClassFromPod(t *testing.T) {
	t.Run("no_containers", func(t *testing.T) {
		p := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{}}}
		got := qosClassFromPod(p)
		if got != "BestEffort" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("best_effort", func(t *testing.T) {
		p := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "app", Resources: corev1.ResourceRequirements{}},
				},
			},
		}
		got := qosClassFromPod(p)
		if got != "BestEffort" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("burstable", func(t *testing.T) {
		p := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "app",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
					},
				},
			},
		}
		got := qosClassFromPod(p)
		if got != "Burstable" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("guaranteed", func(t *testing.T) {
		q := resource.MustParse("100m")
		p := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "app",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    q,
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    q,
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					},
				},
			},
		}
		got := qosClassFromPod(p)
		if got != "Guaranteed" {
			t.Errorf("got %q", got)
		}
	})
}

func TestComputePodSpecHash(t *testing.T) {
	t.Run("stable_for_same_spec", func(t *testing.T) {
		p := &corev1.Pod{
			Spec: corev1.PodSpec{
				ServiceAccountName: "default",
				HostNetwork:        false,
				Containers:         []corev1.Container{{Name: "app", Image: "nginx:1.21"}},
			},
		}
		h1 := computePodSpecHash(p)
		h2 := computePodSpecHash(p)
		if h1 == "" || h2 == "" {
			t.Fatal("hash should be non-empty")
		}
		if h1 != h2 {
			t.Errorf("same spec should yield same hash: %q vs %q", h1, h2)
		}
	})
	t.Run("different_spec_different_hash", func(t *testing.T) {
		p1 := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "a", Image: "x"}}}}
		p2 := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "b", Image: "y"}}}}
		h1 := computePodSpecHash(p1)
		h2 := computePodSpecHash(p2)
		if h1 == h2 {
			t.Errorf("different spec should yield different hash: %q", h1)
		}
	})
	t.Run("hash_is_hex_sha256_length", func(t *testing.T) {
		p := &corev1.Pod{Spec: corev1.PodSpec{}}
		h := computePodSpecHash(p)
		if len(h) != 64 {
			t.Errorf("SHA256 hex should be 64 chars, got %d", len(h))
		}
		for _, c := range h {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("hash should be hex: %q", h)
				break
			}
		}
	})
	t.Run("order_insensitivity_containers_and_volumes", func(t *testing.T) {
		// Same spec, different order of containers and volumes -> hash must equal
		p1 := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "a", Image: "img-a"},
					{Name: "b", Image: "img-b"},
				},
				Volumes: []corev1.Volume{
					{Name: "v2"}, {Name: "v1"},
				},
			},
		}
		p2 := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "b", Image: "img-b"},
					{Name: "a", Image: "img-a"},
				},
				Volumes: []corev1.Volume{
					{Name: "v1"}, {Name: "v2"},
				},
			},
		}
		h1 := computePodSpecHash(p1)
		h2 := computePodSpecHash(p2)
		if h1 != h2 {
			t.Errorf("same spec different order should yield same hash: %q vs %q", h1, h2)
		}
	})
}

func TestBuildPayloadIncludesDeploymentsAndReplicaSets(t *testing.T) {
	replicas := int32(2)
	client := fake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api",
				Namespace: "default",
				UID:       types.UID("deploy-uid"),
				Labels:    map[string]string{"app": "api"},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "api", Image: "api:v1"}},
					},
				},
			},
			Status: appsv1.DeploymentStatus{
				ReadyReplicas:       1,
				AvailableReplicas:   1,
				UnavailableReplicas: 1,
				UpdatedReplicas:     2,
			},
		},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "api-abc123",
				Namespace: "default",
				UID:       types.UID("rs-uid"),
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "Deployment", Name: "api", UID: types.UID("deploy-uid")},
				},
			},
			Spec: appsv1.ReplicaSetSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "api", Image: "api:v1"}},
					},
				},
			},
			Status: appsv1.ReplicaSetStatus{
				ReadyReplicas:        1,
				AvailableReplicas:    1,
				FullyLabeledReplicas: 2,
			},
		},
	)
	s := NewSyncer(
		client,
		"http://core",
		&cluster.Info{ID: "cluster-1", Name: "cluster-1", Source: "test", K8sVersion: "v1.29.15", Distribution: "kubeadm"},
		time.Minute,
		metav1.NamespaceAll,
		"agent-1",
		"node-1",
		"test",
	)

	payload, err := s.buildPayload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Deployments) != 1 {
		t.Fatalf("expected 1 deployment in payload, got %d", len(payload.Data.Deployments))
	}
	if got := payload.Data.Deployments[0]; got.Name != "api" || got.Namespace != "default" || got.UID != "deploy-uid" {
		t.Fatalf("unexpected deployment payload: %#v", got)
	}
	if len(payload.Data.ReplicaSets) != 1 {
		t.Fatalf("expected 1 replicaset in payload, got %d", len(payload.Data.ReplicaSets))
	}
	if got := payload.Data.ReplicaSets[0]; got.Name != "api-abc123" || got.OwnerKind != "Deployment" || got.OwnerName != "api" {
		t.Fatalf("unexpected replicaset payload: %#v", got)
	}
}

func TestBuildPayloadSkipsTerminatingPods(t *testing.T) {
	now := metav1.Now()
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "terminating",
				Namespace:         "default",
				UID:               types.UID("pod-terminating"),
				DeletionTimestamp: &now,
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "default",
				Containers:         []corev1.Container{{Name: "app", Image: "app:v1"}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "running",
				Namespace: "default",
				UID:       types.UID("pod-running"),
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: "default",
				Containers:         []corev1.Container{{Name: "app", Image: "app:v1"}},
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default", UID: types.UID("sa-default")},
		},
	)
	s := NewSyncer(
		client,
		"http://core",
		&cluster.Info{ID: "cluster-1", Name: "cluster-1", Source: "test", K8sVersion: "v1.29.15", Distribution: "kubeadm"},
		time.Minute,
		metav1.NamespaceAll,
		"agent-1",
		"node-1",
		"test",
	)

	payload, err := s.buildPayload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Pods) != 1 {
		t.Fatalf("expected 1 active pod in payload, got %d", len(payload.Data.Pods))
	}
	if got := payload.Data.Pods[0]; got.Name != "running" || got.UID != "pod-running" {
		t.Fatalf("unexpected pod payload: %#v", got)
	}
	if len(payload.Data.ServiceAccounts) != 1 {
		t.Fatalf("expected 1 service account in payload, got %d", len(payload.Data.ServiceAccounts))
	}
	linked := payload.Data.ServiceAccounts[0].LinkedPods
	if len(linked) != 1 || linked[0] != "pod-running" {
		t.Fatalf("expected linkedPods to include only running pod, got %#v", linked)
	}
}
