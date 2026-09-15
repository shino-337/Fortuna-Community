package riskengine

import (
	"context"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/rbacinventory"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestClusterAdminBindingForPod(t *testing.T) {
	const ref = `{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"cluster-admin"}`
	for _, tc := range []struct {
		name, kind, namespace, cluster, subjects, roleRef string
		matched, want, wantErr                            bool
	}{
		{"direct", "ClusterRoleBinding", "", "c", `[{"kind":"ServiceAccount","name":"sa","namespace":"ns"}]`, ref, true, true, false},
		{"namespaced-role-same-name", "RoleBinding", "ns", "c", `[{"kind":"ServiceAccount","name":"sa"}]`, `{"apiGroup":"rbac.authorization.k8s.io","kind":"Role","name":"cluster-admin"}`, true, false, false},
		{"namespace-only", "RoleBinding", "ns", "c", `[{"kind":"ServiceAccount","name":"sa"}]`, ref, true, false, false},
		{"other-namespace-grant", "RoleBinding", "other", "c", `[{"kind":"ServiceAccount","name":"sa","namespace":"ns"}]`, ref, true, false, false},
		{"missing-namespace", "ClusterRoleBinding", "", "c", `[{"kind":"ServiceAccount","name":"sa"}]`, ref, false, false, false},
		{"group", "ClusterRoleBinding", "", "c", `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccounts:ns"}]`, ref, true, true, false},
		{"all-sa", "ClusterRoleBinding", "", "c", `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccounts"}]`, ref, true, true, false},
		{"authenticated", "ClusterRoleBinding", "", "c", `[{"kind":"Group","apiGroup":"rbac.authorization.k8s.io","name":"system:authenticated"}]`, ref, true, true, false},
		{"user", "ClusterRoleBinding", "", "c", `[{"kind":"User","apiGroup":"rbac.authorization.k8s.io","name":"system:serviceaccount:ns:sa"}]`, ref, true, true, false},
		{"foreign", "ClusterRoleBinding", "", "other", `[{"kind":"ServiceAccount","name":"sa","namespace":"ns"}]`, ref, false, false, false},
		{"bad-group", "ClusterRoleBinding", "", "c", `[{"kind":"Group","name":"system:authenticated"}]`, ref, false, false, false},
		{"wrong-role-kind", "ClusterRoleBinding", "", "c", `[{"kind":"ServiceAccount","name":"sa","namespace":"ns"}]`, `{"apiGroup":"rbac.authorization.k8s.io","kind":"Role","name":"cluster-admin"}`, false, false, true},
		{"wrong-api-group", "ClusterRoleBinding", "", "c", `[{"kind":"ServiceAccount","name":"sa","namespace":"ns"}]`, `{"kind":"ClusterRole","name":"cluster-admin"}`, false, false, true},
		{"missing-role", "ClusterRoleBinding", "", "c", `[{"kind":"ServiceAccount","name":"sa","namespace":"ns"}]`, `{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"missing"}`, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			configureRiskEngineTestDB(t, db)
			if err := db.AutoMigrate(&models.Role{}, &models.ClusterRole{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&models.ClusterRole{UID: "role", Name: "cluster-admin", ClusterID: "c", Rules: `[{"verbs":["*"],"resources":["*"],"apiGroups":["*"]}]`}).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&models.Role{UID: "local-role", Name: "cluster-admin", Namespace: "ns", ClusterID: "c", Rules: `[]`}).Error; err != nil {
				t.Fatal(err)
			}
			var binding any = &models.ClusterRoleBinding{UID: "binding", Name: "grant", ClusterID: tc.cluster, Subjects: tc.subjects, RoleRef: tc.roleRef}
			if tc.kind == "RoleBinding" {
				binding = &models.RoleBinding{UID: "binding", Name: "grant", ClusterID: tc.cluster, Namespace: tc.namespace, Subjects: tc.subjects, RoleRef: tc.roleRef}
			}
			if err := db.Create(binding).Error; err != nil {
				t.Fatal(err)
			}
			got, err := clusterAdminBindingForPod(context.Background(), db, "c", "ns", "sa")
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("risk=%v error=%v", got, err)
			}
			grants, err := rbacinventory.Resolve(db, &models.ServiceAccount{ClusterID: "c", Namespace: "ns", Name: "sa"})
			if (err != nil) != tc.wantErr {
				t.Fatal(err)
			}
			if !tc.wantErr {
				if grants.HasClusterAdminBinding() != got || (len(grants.RoleBindings)+len(grants.ClusterRoleBindings) > 0) != tc.matched {
					t.Fatalf("inconsistent grants: %+v", grants)
				}
				for _, rule := range grants.EffectiveRules {
					if tc.kind == "RoleBinding" && (rule.Scope != "namespace" || rule.Namespace != tc.namespace) {
						t.Fatalf("lost namespace: %+v", rule)
					}
				}
			}
		})
	}
}
