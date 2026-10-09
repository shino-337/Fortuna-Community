package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/rbac"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&models.Role{}, &models.RoleBinding{}, &models.ClusterRole{}, &models.ClusterRoleBinding{}); err != nil {
		t.Fatalf("failed to automigrate: %v", err)
	}
	return db
}

func capabilityIDs(caps []Capability) map[string]struct{} {
	out := make(map[string]struct{})
	for _, c := range caps {
		out[c.ID] = struct{}{}
	}
	return out
}

func TestPCEUpsertsUseClusterQualifiedPodIdentity(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.PodCapability{}, &models.PodRiskProfile{}); err != nil {
		t.Fatal(err)
	}
	a := models.Pod{ClusterID: "cluster-a", UID: "same", Namespace: "ns", HostPID: true}
	b := models.Pod{ClusterID: "cluster-b", UID: "same", Namespace: "ns"}
	for _, pod := range []models.Pod{a, b} {
		if err := upsertPodRiskProfile(db, pod, []Capability{{ID: "CAP", Group: "API", Severity: "HIGH", Evidence: map[string]interface{}{}}}); err != nil {
			t.Fatal(err)
		}
		if err := upsertCapabilities(db, pod, []Capability{{ID: "CAP", Group: "API", Severity: "HIGH", Evidence: map[string]interface{}{}}}); err != nil {
			t.Fatal(err)
		}
	}
	var profiles []models.PodRiskProfile
	if err := db.Order("cluster_id").Find(&profiles).Error; err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || profiles[0].ClusterID != "cluster-a" || profiles[0].StaticRisk != 40 || profiles[1].ClusterID != "cluster-b" || profiles[1].StaticRisk != 0 {
		t.Fatalf("cross-cluster risk profile collision: %+v", profiles)
	}
	var capabilities []models.PodCapability
	if err := db.Order("cluster_id").Find(&capabilities).Error; err != nil {
		t.Fatal(err)
	}
	if len(capabilities) != 2 || capabilities[0].ClusterID != "cluster-a" || capabilities[1].ClusterID != "cluster-b" {
		t.Fatalf("cross-cluster capability collision: %+v", capabilities)
	}
}

// TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch ensures async PCE discards when spec_hash changed (race protection).
func TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	pod := &models.Pod{
		ClusterID: "c1",
		UID:       "u1",
		Name:      "p",
		Namespace: "default",
		SpecHash:  "hashA",
	}
	err := EvaluateAndUpsertPod(ctx, db, pod, "hashB")
	if err != nil {
		t.Fatalf("expected nil (skip): %v", err)
	}
	// Should have returned early without writing (no pod_capabilities table etc.)
}

func TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	pod := &models.Pod{
		ClusterID:                 "c1",
		UID:                       "pod-1",
		Name:                      "p1",
		Namespace:                 "default",
		ServiceAccount:            "default",
		HostNetwork:               true,
		HostPID:                   true,
		HostIPC:                   false,
		ContainerSecurityContexts: `{"app":{"privileged":true}}`,
		Volumes:                   `[{"name":"host","hostPath":{"path":"/"}}]`,
		VolumeMounts:              `[{"name":"host","mountPath":"/"}]`,
	}

	caps, err := EvaluatePod(ctx, db, pod)
	if err != nil {
		t.Fatalf("EvaluatePod error: %v", err)
	}
	got := capabilityIDs(caps)

	expected := []string{
		NET_HOSTNETWORK,
		ESC_HOSTPID_POD,
		ESC_PRIV_POD,
		ESC_HOSTPATH_NODE,
		ESC_RUNTIME_PROBE,
		ID_TOKEN_POD,
	}
	for _, id := range expected {
		if _, ok := got[id]; !ok {
			t.Fatalf("expected capability %s, got %v", id, got)
		}
	}
}

func TestEvaluatePod_AutomountFalse_NoTokenSteal(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	auto := false
	pod := &models.Pod{
		ClusterID:                    "c1",
		UID:                          "pod-2",
		Name:                         "p2",
		Namespace:                    "default",
		ServiceAccount:               "default",
		AutomountServiceAccountToken: &auto,
	}

	caps, err := EvaluatePod(ctx, db, pod)
	if err != nil {
		t.Fatalf("EvaluatePod error: %v", err)
	}
	got := capabilityIDs(caps)
	if _, ok := got[ID_TOKEN_POD]; ok {
		t.Fatalf("did not expect ID_TOKEN_POD when automount is false")
	}
	if len(got) != 0 {
		t.Fatalf("expected no capabilities, got %v", got)
	}
}

func TestEvaluatePod_ControlPlaneNamespace(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	pod := &models.Pod{
		ClusterID:      "c1",
		UID:            "pod-3",
		Name:           "p3",
		Namespace:      "kube-system",
		ServiceAccount: "default",
	}

	caps, err := EvaluatePod(ctx, db, pod)
	if err != nil {
		t.Fatalf("EvaluatePod error: %v", err)
	}
	got := capabilityIDs(caps)
	if _, ok := got[CTRL_CONTROL_PLANE_POD]; !ok {
		t.Fatalf("expected CTRL_CONTROL_PLANE_POD, got %v", got)
	}
}

func TestEvaluatePod_RuntimeProbeSensitiveHostPath(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	pod := &models.Pod{
		ClusterID:      "c1",
		UID:            "pod-5",
		Name:           "p5",
		Namespace:      "default",
		ServiceAccount: "default",
		Volumes:        `[{"name":"proc","hostPath":{"path":"/proc"}}]`,
		VolumeMounts:   `[{"name":"proc","mountPath":"/host/proc","readOnly":true}]`,
	}

	caps, err := EvaluatePod(ctx, db, pod)
	if err != nil {
		t.Fatalf("EvaluatePod error: %v", err)
	}
	got := capabilityIDs(caps)
	if _, ok := got[ESC_RUNTIME_PROBE]; !ok {
		t.Fatalf("expected ESC_RUNTIME_PROBE, got %v", got)
	}
}

func TestEvaluatePod_APIWriteAccess(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	roleRules, _ := json.Marshal([]map[string]interface{}{
		{"verbs": []string{"get", "list", "create"}},
	})
	if err := db.Create(&models.Role{
		ClusterID: "c1",
		Name:      "writer",
		Namespace: "ns",
		UID:       "role-1",
		Rules:     string(roleRules),
	}).Error; err != nil {
		t.Fatalf("failed to create role: %v", err)
	}

	subjects, _ := json.Marshal([]map[string]interface{}{
		{"kind": "ServiceAccount", "name": "default", "namespace": "ns"},
	})
	roleRef, _ := json.Marshal(map[string]interface{}{
		"kind": "Role",
		"name": "writer",
	})
	if err := db.Create(&models.RoleBinding{
		ClusterID: "c1",
		Name:      "rb-1",
		Namespace: "ns",
		UID:       "rb-1",
		RoleRef:   string(roleRef),
		Subjects:  string(subjects),
	}).Error; err != nil {
		t.Fatalf("failed to create rolebinding: %v", err)
	}

	pod := &models.Pod{
		ClusterID:      "c1",
		UID:            "pod-4",
		Name:           "p4",
		Namespace:      "ns",
		ServiceAccount: "default",
	}

	caps, err := EvaluatePod(ctx, db, pod)
	if err != nil {
		t.Fatalf("EvaluatePod error: %v", err)
	}
	got := capabilityIDs(caps)
	if _, ok := got[API_RBAC_WRITE_CLUSTER]; !ok {
		t.Fatalf("expected API_RBAC_WRITE_CLUSTER, got %v", got)
	}
}

func TestHasWriteVerbs_NoWrite(t *testing.T) {
	rulesJSON := `[{"verbs":["get","list","watch"]}]`
	if rbac.HasWriteVerbs(rulesJSON) {
		t.Fatalf("expected HasWriteVerbs to be false for read-only verbs")
	}
}

func TestHasHostPathMount_NoHostPath(t *testing.T) {
	volumesJSON := `[{"name":"config","configMap":{"name":"cfg"}}]`
	mountsJSON := `[{"name":"config","mountPath":"/etc/config"}]`
	if escape, _ := hasEscapeHostPathMount(volumesJSON, mountsJSON); escape {
		t.Fatalf("expected hasEscapeHostPathMount to be false when no hostPath volume")
	}
}

func TestHasEscapeHostPathMount(t *testing.T) {
	for _, tc := range []struct {
		name     string
		hostPath string
		readOnly bool
		want     bool
	}{
		{"ro localtime", "/etc/localtime", true, false},
		{"ro zoneinfo", "/usr/share/zoneinfo", true, false},
		{"ro zoneinfo file", "/usr/share/zoneinfo/UTC", true, false},
		{"ro ca certs", "/etc/ssl/certs", true, false},
		{"ro var log", "/var/log", true, false},
		{"rw localtime", "/etc/localtime", false, true},
		{"rw arbitrary dir", "/data/cache", false, true},
		{"ro host root", "/", true, true},
		{"ro etc", "/etc", true, true},
		{"ro kubernetes pki", "/etc/kubernetes/pki", true, true},
		{"ro kubelet", "/var/lib/kubelet", true, true},
		{"ro var (parent of kubelet)", "/var", true, true},
		{"ro proc", "/proc", true, true},
		{"ro sys", "/sys", true, true},
		{"ro root home", "/root", true, true},
		{"ro home", "/home", true, true},
		{"ro run", "/run", true, true},
		{"ro var run", "/var/run", true, true},
		{"ro docker sock", "/var/run/docker.sock", true, true},
		{"ro containerd sock", "/run/containerd/containerd.sock", true, true},
		{"ro crio sock", "/var/run/crio/crio.sock", true, true},
		{"ro runtime-ish name", "/runtime-data", true, false},
		{"relative", "data", false, false},
	} {
		vols := `[{"name":"v","hostPath":{"path":"` + tc.hostPath + `"}}]`
		mounts := fmt.Sprintf(`[{"name":"v","mountPath":"/mnt","readOnly":%v}]`, tc.readOnly)
		got, matches := hasEscapeHostPathMount(vols, mounts)
		if got != tc.want {
			t.Errorf("%s: got %v want %v (matches=%v)", tc.name, got, tc.want, matches)
		}
	}
	if !isContainerRuntimeSocket("/run/containerd/containerd.sock") || isContainerRuntimeSocket("/run/containerd") {
		t.Fatal("isContainerRuntimeSocket mismatch")
	}
}

func TestEvaluatePod_ReadOnlyLocaltimeIsNotHostPathEscape(t *testing.T) {
	db := newTestDB(t)
	pod := &models.Pod{
		ClusterID:      "c1",
		UID:            "pod-tz",
		Name:           "tz",
		Namespace:      "default",
		ServiceAccount: "default",
		Volumes:        `[{"name":"tz","hostPath":{"path":"/etc/localtime"}}]`,
		VolumeMounts:   `[{"name":"tz","mountPath":"/etc/localtime","readOnly":true}]`,
	}
	caps, err := EvaluatePod(context.Background(), db, pod)
	if err != nil {
		t.Fatalf("EvaluatePod error: %v", err)
	}
	if _, ok := capabilityIDs(caps)[ESC_HOSTPATH_NODE]; ok {
		t.Fatalf("read-only /etc/localtime must not be ESC_HOSTPATH_NODE: %v", capabilityIDs(caps))
	}
}
