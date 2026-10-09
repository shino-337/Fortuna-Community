package graph

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
)

const testPodNS = "payments"

func testPodAndSA() (models.Pod, models.ServiceAccount) {
	pod := models.Pod{UID: "pod-1", Name: "api", Namespace: testPodNS, NodeName: "worker-1", ServiceAccount: "api"}
	sa := models.ServiceAccount{UID: "sa-1", Name: "api", Namespace: testPodNS}
	return pod, sa
}

func buildTestPaths(pod models.Pod, sa models.ServiceAccount, caps []models.PodCapability, steps []models.PodAttackStep,
	rbs []models.RoleBinding, crbs []models.ClusterRoleBinding, roles map[string]models.Role, crs map[string]models.ClusterRole) []AttackPath {
	return buildDeterministicPaths(pod, sa, nil, map[string]models.ServiceAccount{}, rbs, crbs, roles, crs,
		caps, steps, ReachabilityContext{Now: time.Now()}, true)
}

func pathToNode(paths []AttackPath, nodeID string) *AttackPath {
	for i := range paths {
		n := paths[i].Nodes
		if len(n) > 0 && n[len(n)-1].ID == nodeID {
			return &paths[i]
		}
	}
	return nil
}

// A1 + A2: escape capabilities of a pod outside kube-system must reach its
// (cluster-scoped) node; hostIPC alone is not an escape.
func TestBuildDeterministicPaths_EscapeCapabilityReachesNode(t *testing.T) {
	pod, sa := testPodAndSA()
	for _, tc := range []struct {
		capID    string
		wantNode bool
	}{
		{"ESC_HOSTPATH_NODE", true},
		{"ESC_RUNTIME_ACTIVE", true},
		{"ESC_PRIV_POD", true},
		{"ESC_HOSTPID_POD", true},
		{"ESC_RUNTIME_PROC_ROOT", true},
		{"ESC_HOSTIPC_POD", false},
	} {
		caps := []models.PodCapability{{PodUID: pod.UID, CapabilityID: tc.capID, State: "detected"}}
		paths := buildTestPaths(pod, sa, caps, nil, nil, nil, nil, nil)
		p := pathToNode(paths, "node:worker-1")
		if !tc.wantNode {
			if p != nil {
				t.Errorf("%s: unexpected path to node", tc.capID)
			}
			continue
		}
		if p == nil {
			t.Fatalf("%s: no path from %s pod to its node; paths=%+v", tc.capID, testPodNS, paths)
		}
		if p.Explainability == nil || p.Explainability.Class != PathClassEscape {
			t.Errorf("%s: want ESCAPE_PATH, got %+v", tc.capID, p.Explainability)
		}
		if ns := p.Nodes[len(p.Nodes)-1].Properties["namespace"]; ns != "" {
			t.Errorf("%s: node must be cluster-scoped, got namespace %v", tc.capID, ns)
		}
	}
}

func TestBuildDeterministicPaths_PrivilegedStrongerThanHostPID(t *testing.T) {
	pod, sa := testPodAndSA()
	strength := func(capID string) float64 {
		caps := []models.PodCapability{{PodUID: pod.UID, CapabilityID: capID, State: "detected"}}
		p := pathToNode(buildTestPaths(pod, sa, caps, nil, nil, nil, nil, nil), "node:worker-1")
		if p == nil {
			t.Fatalf("%s: no node path", capID)
		}
		return p.Explainability.Strength
	}
	if priv, pid := strength("ESC_PRIV_POD"), strength("ESC_HOSTPID_POD"); priv <= pid {
		t.Fatalf("privileged (%v) should be a stronger escape than hostPID (%v)", priv, pid)
	}
}

// A5: a node-level attack step leads to the node, so the step edge shows up in paths.
func TestBuildDeterministicPaths_NodeAttackStepReachesNode(t *testing.T) {
	pod, sa := testPodAndSA()
	steps := []models.PodAttackStep{{PodUID: pod.UID, StepID: "NODE_CRED_DUMP", Confidence: 0.9}}
	p := pathToNode(buildTestPaths(pod, sa, nil, steps, nil, nil, nil, nil), "node:worker-1")
	if p == nil {
		t.Fatal("expected pod → NODE_CRED_DUMP step → node path")
	}
	if p.Edges[0].Type != EdgeTypeCanStealCredential {
		t.Fatalf("first edge = %s, want %s", p.Edges[0].Type, EdgeTypeCanStealCredential)
	}
	if len(p.Explainability.Evidence.AttackSteps) != 1 {
		t.Fatalf("attack step evidence = %v", p.Explainability.Evidence.AttackSteps)
	}

	// Steps that do not act on the node stay off node paths.
	steps = []models.PodAttackStep{{PodUID: pod.UID, StepID: "RBAC_ABUSE", Confidence: 0.9}}
	if pathToNode(buildTestPaths(pod, sa, nil, steps, nil, nil, nil, nil), "node:worker-1") != nil {
		t.Fatal("RBAC_ABUSE step must not lead to the node")
	}
}

// A3: same-named Roles in different namespaces and a same-named ClusterRole
// must be distinct graph nodes.
func TestAddRBACChainForSA_RoleIDsIncludeKindAndNamespace(t *testing.T) {
	sa := models.ServiceAccount{UID: "sa-1", Name: "api", Namespace: "ns-a"}
	rules := `[{"verbs":["get"],"resources":["secrets"],"apiGroups":[""]}]`
	roles := map[string]models.Role{
		"ns-a/foo": {Name: "foo", Namespace: "ns-a", Rules: rules},
		"ns-b/foo": {Name: "foo", Namespace: "ns-b", Rules: rules},
	}
	crs := map[string]models.ClusterRole{"foo": {Name: "foo", Rules: rules}}
	subj := `[{"kind":"ServiceAccount","name":"api","namespace":"ns-a"}]`
	rbs := []models.RoleBinding{
		{Name: "rb-a", Namespace: "ns-a", RoleRef: `{"kind":"Role","name":"foo"}`, Subjects: subj},
		{Name: "rb-b", Namespace: "ns-b", RoleRef: `{"kind":"Role","name":"foo"}`, Subjects: subj},
		{Name: "rb-c", Namespace: "ns-b", RoleRef: `{"kind":"ClusterRole","name":"foo"}`, Subjects: subj},
	}
	crbs := []models.ClusterRoleBinding{{Name: "crb", RoleRef: `{"kind":"ClusterRole","name":"foo"}`, Subjects: subj}}
	g := NewAttackGraph()
	addRBACChainForSA(g, sa, rbs, crbs, roles, crs)
	for _, id := range []string{"role:Role:ns-a/foo", "role:Role:ns-b/foo", "role:ClusterRole:ns-b/foo", "role:ClusterRole:foo"} {
		n, ok := g.Nodes[id]
		if !ok {
			t.Errorf("missing role node %s; nodes=%v", id, g.Nodes)
			continue
		}
		if n.Label != "foo" || n.PrivilegeLevel != 2 {
			t.Errorf("%s: label=%q level=%d", id, n.Label, n.PrivilegeLevel)
		}
	}
	if got := roleNameFromNodeID("role:ClusterRole:ns-b/foo"); got != "foo" {
		t.Errorf("roleNameFromNodeID = %q", got)
	}
	if got := roleNameFromNodeID("role:cluster-admin"); got != "cluster-admin" {
		t.Errorf("legacy roleNameFromNodeID = %q", got)
	}
}

// A4: subject namespace defaults to the RoleBinding namespace only; SA groups match.
func TestBindingRefersToSA(t *testing.T) {
	sa := models.ServiceAccount{Name: "api", Namespace: "ns-a"}
	for _, tc := range []struct {
		name, subjects, bindingNS string
		want                      bool
	}{
		{"explicit ns", `[{"kind":"ServiceAccount","name":"api","namespace":"ns-a"}]`, "ns-b", true},
		{"other ns", `[{"kind":"ServiceAccount","name":"api","namespace":"ns-b"}]`, "ns-b", false},
		{"empty ns defaults to RB ns", `[{"kind":"ServiceAccount","name":"api"}]`, "ns-a", true},
		{"empty ns in other RB ns", `[{"kind":"ServiceAccount","name":"api"}]`, "ns-b", false},
		{"empty ns in CRB", `[{"kind":"ServiceAccount","name":"api"}]`, "", false},
		{"all SAs group", `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccounts"}]`, "", true},
		{"ns SAs group", `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccounts:ns-a"}]`, "", true},
		{"other ns SAs group", `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccounts:ns-b"}]`, "", false},
		{"authenticated group", `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:authenticated"}]`, "", true},
		{"SA user name", `[{"kind":"User","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccount:ns-a:api"}]`, "", true},
		{"invalid", `not json`, "ns-a", false},
	} {
		if got := bindingRefersToSA(tc.subjects, tc.bindingNS, sa); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

// B7: a role name containing "admin" no longer makes a path PRIV_ESC; rules do.
func TestBuildDeterministicPaths_RoleClassFromRulesNotName(t *testing.T) {
	pod, sa := testPodAndSA()
	subj := `[{"kind":"ServiceAccount","name":"api"}]`
	build := func(roleName, rules string) *AttackPath {
		roles := map[string]models.Role{testPodNS + "/" + roleName: {Name: roleName, Namespace: testPodNS, Rules: rules}}
		rbs := []models.RoleBinding{{Name: "rb", Namespace: testPodNS, RoleRef: `{"kind":"Role","name":"` + roleName + `"}`, Subjects: subj}}
		return pathToNode(buildTestPaths(pod, sa, nil, nil, rbs, nil, roles, nil), rbacRoleNodeID("Role", testPodNS, roleName))
	}
	readSecrets := build("secret-admin", `[{"verbs":["get","list"],"resources":["secrets"],"apiGroups":[""]}]`)
	if readSecrets == nil || readSecrets.Explainability.Class != PathClassDataExfil {
		t.Fatalf("secret reader named *admin should be DATA_EXFIL, got %+v", readSecrets)
	}
	createPods := build("ci-runner", `[{"verbs":["create"],"resources":["pods"],"apiGroups":[""]}]`)
	if createPods == nil || createPods.Explainability.Class != PathClassPrivEsc {
		t.Fatalf("pod creator should be PRIV_ESC, got %+v", createPods)
	}
	norm := normalizePaths([]AttackPath{*readSecrets})
	if len(norm) != 1 || norm[0].TargetPrivilege != 2 {
		t.Fatalf("normalized privilege = %+v", norm)
	}
	if got := classifyObjectiveByRoleTarget(pathNormalized{TargetPrivilege: 2, Target: pathEndpoint{ID: "role:Role:ns/secret-admin"}}); got != "SECRET_EXFIL" {
		t.Fatalf("objective = %s, want SECRET_EXFIL", got)
	}
}

func TestRelationalEdgeFeasible(t *testing.T) {
	pod := GraphNode{Namespace: "a"}
	if !relationalEdgeFeasible(GraphEdge{Type: EdgeTypeLateralMove}, pod, clusterNodeGraphNode("n1")) {
		t.Fatal("edges into cluster-scoped nodes must be feasible")
	}
	if relationalEdgeFeasible(GraphEdge{Type: EdgeTypeLateralMove}, pod, GraphNode{Namespace: "b"}) {
		t.Fatal("cross-namespace non-RBAC edge must be rejected")
	}
	if !relationalEdgeFeasible(GraphEdge{Type: EdgeTypeGrantsRole}, pod, GraphNode{Namespace: "b"}) {
		t.Fatal("cross-namespace RBAC edge must be feasible")
	}
}
