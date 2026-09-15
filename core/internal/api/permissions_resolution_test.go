package api

import (
	"github.com/fortuna/core/pkg/models"
	rbacv1 "k8s.io/api/rbac/v1"
	"testing"
)

func TestServiceAccountRBACResolution(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	sa := models.ServiceAccount{ID: 1, ClusterID: "a", Namespace: "shared", Name: "shared"}
	subjects := `[{"kind":"ServiceAccount","name":"shared","namespace":"shared"},{"kind":"ServiceAccount","name":"shared","namespace":"shared"}]`
	for _, row := range []any{
		&models.Role{ClusterID: "a", Namespace: "target", Name: "reader", UID: "role", Rules: `[{"verbs":["delete"],"resources":["secrets"]}]`},
		&models.ClusterRole{ClusterID: "a", Name: "reader", UID: "cr", Rules: `[{"verbs":["get"],"resources":["pods"],"apiGroups":[""]},{"verbs":["get"],"nonResourceURLs":["/healthz"]}]`},
		&models.ClusterRole{ClusterID: "b", Name: "reader", UID: "cr-b", Rules: `[{"verbs":["*"],"resources":["*"]}]`},
		&models.RoleBinding{ClusterID: "a", Namespace: "target", UID: "rb", Name: "reader", Subjects: subjects, RoleRef: `{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"reader"}`},
		&models.ClusterRoleBinding{ClusterID: "a", UID: "crb", Name: "missing-ns", Subjects: `[{"kind":"ServiceAccount","name":"shared"}]`, RoleRef: `{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"reader"}`},
	} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	out, err := buildServiceAccountPermissions(db, &sa)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.RoleBindings) != 1 || out.RoleBindings[0].ClusterRole == nil || out.RoleBindings[0].Role != nil || len(out.ClusterRoleBindings) != 0 || len(out.EffectiveRules) != 1 {
		t.Fatalf("unexpected grants: %+v", out)
	}
	rule := out.EffectiveRules[0]
	if rule.Scope != "namespace" || rule.Namespace != "target" || rule.Verbs[0] != "get" || rule.Resources[0] != "pods" {
		t.Fatalf("wrong scope/rules: %+v", rule)
	}
	if err := db.Create(&models.ClusterRoleBinding{ClusterID: "a", UID: "group", Name: "group", Subjects: `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccounts:shared"}]`, RoleRef: `{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"reader"}`}).Error; err != nil {
		t.Fatal(err)
	}
	out, err = buildServiceAccountPermissions(db, &sa)
	if err != nil || len(out.ClusterRoleBindings) != 1 || len(out.EffectiveRules) != 3 || out.EffectiveRules[2].Scope != "cluster" || len(out.EffectiveRules[2].NonResourceURLs) != 1 {
		t.Fatalf("group grants: %+v %v", out, err)
	}
	if err := db.Model(&models.RoleBinding{}).Where("uid = ?", "rb").Update("role_ref", `{"apiGroup":"rbac.authorization.k8s.io","kind":"Role","name":"missing"}`).Error; err != nil {
		t.Fatal(err)
	}
	w := request("GET", "/inventory/serviceaccounts/sa-a/permissions", "a", "")
	if w.Code != 500 {
		t.Fatalf("missing role must not appear as empty permissions: %d %s", w.Code, w.Body)
	}
	if err := db.Migrator().DropTable(&models.RoleBinding{}); err != nil {
		t.Fatal(err)
	}
	w = request("GET", "/inventory/serviceaccounts/sa-a/permissions", "a", "")
	if w.Code != 500 {
		t.Fatalf("storage failure: %d %s", w.Code, w.Body)
	}
}

func TestServiceAccountSubjectMatching(t *testing.T) {
	sa := &models.ServiceAccount{Name: "app", Namespace: "team"}
	for _, tc := range []struct {
		subject   rbacv1.Subject
		namespace string
		want      bool
	}{
		{rbacv1.Subject{Kind: "ServiceAccount", Name: "app"}, "team", true},
		{rbacv1.Subject{Kind: "ServiceAccount", Name: "app"}, "", false},
		{rbacv1.Subject{Kind: "ServiceAccount", Name: "app", Namespace: "other"}, "team", false},
		{rbacv1.Subject{Kind: "Group", APIGroup: rbacv1.GroupName, Name: "system:authenticated"}, "", true},
		{rbacv1.Subject{Kind: "Group", APIGroup: rbacv1.GroupName, Name: "system:serviceaccounts:other"}, "", false},
		{rbacv1.Subject{Kind: "User", APIGroup: rbacv1.GroupName, Name: "system:serviceaccount:team:app"}, "", true},
	} {
		if got := serviceAccountSubjectMatches(tc.subject, tc.namespace, sa); got != tc.want {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
}
