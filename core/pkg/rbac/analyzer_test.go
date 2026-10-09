package rbac

import "testing"

func TestClassifyRoleRisk_ExecAndNodeProxy(t *testing.T) {
	for _, tc := range []struct {
		name, rules, want string
	}{
		{"get nodes/proxy", `[{"verbs":["get"],"resources":["nodes/proxy"],"apiGroups":[""]}]`, "critical"},
		{"create nodes/proxy", `[{"verbs":["create"],"resources":["nodes/proxy"],"apiGroups":[""]}]`, "critical"},
		{"get pods/exec (websocket)", `[{"verbs":["get"],"resources":["pods/exec"],"apiGroups":[""]}]`, "high"},
		{"create pods/exec", `[{"verbs":["create"],"resources":["pods/exec"],"apiGroups":[""]}]`, "high"},
		{"get pods/attach", `[{"verbs":["get"],"resources":["pods/attach"],"apiGroups":[""]}]`, "high"},
		{"get nodes", `[{"verbs":["get","list"],"resources":["nodes"],"apiGroups":[""]}]`, "none"},
		{"list pods", `[{"verbs":["get","list"],"resources":["pods"],"apiGroups":[""]}]`, "none"},
	} {
		if got := ClassifyRoleRisk("custom", tc.rules); got != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestClassifyRoleRisk_NameDoesNotRateRole(t *testing.T) {
	readOnly := `[{"verbs":["get","list"],"resources":["configmaps"],"apiGroups":[""]}]`
	for _, name := range []string{"configmap-admin", "admin-viewer", "my-cluster-admin-readonly"} {
		if got := ClassifyRoleRisk(name, readOnly); got != "none" {
			t.Errorf("%s with read-only configmap rules: got %s want none", name, got)
		}
	}
	if got := ClassifyRoleRisk("cluster-admin", ""); got != "critical" {
		t.Errorf("built-in cluster-admin: got %s", got)
	}
	if got := ClassifyRoleRisk("anything", `[{"verbs":["*"],"resources":["*"],"apiGroups":["*"]}]`); got != "critical" {
		t.Errorf("*/*: got %s", got)
	}
}

func TestDeriveCapabilities_NodeProxyAndExec(t *testing.T) {
	has := func(caps []DerivedCapability, typ string) bool {
		for _, c := range caps {
			if c.Type == typ {
				return true
			}
		}
		return false
	}
	if caps := DeriveCapabilities(`[{"verbs":["get","list"],"resources":["nodes"],"apiGroups":[""]}]`); has(caps, "NODE_PROXY") {
		t.Errorf("get nodes must not be NODE_PROXY: %+v", caps)
	}
	if caps := DeriveCapabilities(`[{"verbs":["get"],"resources":["nodes/proxy"],"apiGroups":[""]}]`); !has(caps, "NODE_PROXY") {
		t.Errorf("get nodes/proxy must be NODE_PROXY: %+v", caps)
	}
	if caps := DeriveCapabilities(`[{"verbs":["patch"],"resources":["nodes/proxy"],"apiGroups":[""]}]`); !has(caps, "NODE_PROXY") {
		t.Errorf("any verb on nodes/proxy must be NODE_PROXY: %+v", caps)
	}
	if caps := DeriveCapabilities(`[{"verbs":["get"],"resources":["pods/attach"],"apiGroups":[""]}]`); !has(caps, "EXEC_ACCESS") {
		t.Errorf("get pods/attach must be EXEC_ACCESS: %+v", caps)
	}
}

func TestRolePrivilegeLevel(t *testing.T) {
	for _, tc := range []struct {
		name, rules string
		want        int
	}{
		{"missing", "", 0},
		{"invalid", "nope", 0},
		{"wildcard", `[{"verbs":["*"],"resources":["*"],"apiGroups":["*"]}]`, 5},
		{"bind", `[{"verbs":["bind"],"resources":["clusterroles"],"apiGroups":["rbac.authorization.k8s.io"]}]`, 5},
		{"create pods", `[{"verbs":["create"],"resources":["pods"],"apiGroups":[""]}]`, 4},
		{"exec", `[{"verbs":["get"],"resources":["pods/exec"],"apiGroups":[""]}]`, 4},
		{"node proxy", `[{"verbs":["get"],"resources":["nodes/proxy"],"apiGroups":[""]}]`, 4},
		{"write rolebindings", `[{"verbs":["create"],"resources":["rolebindings"],"apiGroups":["rbac.authorization.k8s.io"]}]`, 4},
		{"write configmaps", `[{"verbs":["update"],"resources":["configmaps"],"apiGroups":[""]}]`, 3},
		{"read secrets", `[{"verbs":["get"],"resources":["secrets"],"apiGroups":[""]}]`, 2},
		{"read pods", `[{"verbs":["get"],"resources":["pods"],"apiGroups":[""]}]`, 1},
	} {
		if got := RolePrivilegeLevel(tc.rules); got != tc.want {
			t.Errorf("%s: got %d want %d", tc.name, got, tc.want)
		}
	}
}
