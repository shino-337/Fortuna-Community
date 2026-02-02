package capability

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fortuna/core/pkg/models"
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

func TestEvaluatePod_PrivilegedHostPathNetworkKernelAutomount(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	pod := &models.Pod{
		ClusterID:      "c1",
		UID:            "pod-1",
		Name:           "p1",
		Namespace:      "default",
		ServiceAccount: "default",
		HostNetwork:    true,
		HostPID:        true,
		HostIPC:        false,
		ContainerSecurityContexts: `{"app":{"privileged":true}}`,
		Volumes: `[{"name":"host","hostPath":{"path":"/"}}]`,
		VolumeMounts: `[{"name":"host","mountPath":"/"}]`,
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
		ClusterID:                   "c1",
		UID:                         "pod-2",
		Name:                        "p2",
		Namespace:                   "default",
		ServiceAccount:              "default",
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
	if hasWriteVerbs(rulesJSON) {
		t.Fatalf("expected hasWriteVerbs to be false for read-only verbs")
	}
}

func TestHasHostPathMount_NoHostPath(t *testing.T) {
	volumesJSON := `[{"name":"config","configMap":{"name":"cfg"}}]`
	mountsJSON := `[{"name":"config","mountPath":"/etc/config"}]`
	if hasHostPathMount(volumesJSON, mountsJSON) {
		t.Fatalf("expected hasHostPathMount to be false when no hostPath volume")
	}
}
