package authorization_test

import (
	"testing"

	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

func TestNormalizeRole_UserToOperator(t *testing.T) {
	if got := authorization.NormalizeRole(models.RoleUser); got != models.RoleOperator {
		t.Fatalf("got %q want %q", got, models.RoleOperator)
	}
	if got := authorization.NormalizeRole(models.RoleClusterAdmin); got != models.RoleClusterAdmin {
		t.Fatalf("got %q want %q", got, models.RoleClusterAdmin)
	}
}

func TestViewerInvestigationsReadOnly(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleViewer)
	if !authorization.HasPermission(perms, authorization.PermissionInvestigationsRead) {
		t.Fatal("viewer needs investigations.read")
	}
	if authorization.HasPermission(perms, authorization.PermissionInvestigationsWrite) {
		t.Fatal("viewer must not have investigations.write")
	}
	if authorization.HasPermission(perms, authorization.PermissionInvestigationsDelete) {
		t.Fatal("viewer must not have investigations.delete")
	}
}

func TestOperatorInvestigationsWriteNotAdminAudit(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleOperator)
	if !authorization.HasPermission(perms, authorization.PermissionInvestigationsWrite) {
		t.Fatal("operator needs investigations.write")
	}
	if authorization.HasPermission(perms, authorization.PermissionSystemAuditRead) {
		t.Fatal("operator must not have system.audit.read")
	}
}

func TestViewerCannotMutateFindingsOrTraverseGraph(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleViewer)
	if authorization.HasPermission(perms, authorization.PermissionFindingsAck) {
		t.Fatal("viewer must not have findings.ack")
	}
	if !authorization.HasPermission(perms, authorization.PermissionGraphReadSummary) {
		t.Fatal("viewer needs graph.read.summary")
	}
}

func TestOperatorCannotRegisterUsers(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleOperator)
	if authorization.HasPermission(perms, authorization.PermissionAuthRegister) {
		t.Fatal("operator must not register users")
	}
}

func TestAdminHasAll(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleAdmin)
	all := authorization.AllPermissions()
	if len(perms) != len(all) {
		t.Fatalf("admin permission count %d != all %d", len(perms), len(all))
	}
}

func TestRetiredUserAdminRoleHasNoPermissions(t *testing.T) {
	// user_admin was removed; accounts left with that role must get nothing.
	if perms := authorization.PermissionsForUser("user_admin"); len(perms) != 0 {
		t.Fatalf("retired user_admin role must have no permissions, got %v", perms)
	}
}

func TestRoleLadderViewerOperatorClusterAdmin(t *testing.T) {
	viewer := authorization.PermissionsForRole(models.RoleViewer)
	operator := authorization.PermissionsForRole(models.RoleOperator)
	clusterAdmin := authorization.PermissionsForRole(models.RoleClusterAdmin)
	for _, p := range viewer {
		if !authorization.HasPermission(operator, p) {
			t.Fatalf("operator must hold every viewer permission, missing %s", p)
		}
	}
	for _, p := range operator {
		if !authorization.HasPermission(clusterAdmin, p) {
			t.Fatalf("cluster_admin must hold every operator permission, missing %s", p)
		}
	}
	if len(clusterAdmin) <= len(operator) || len(operator) <= len(viewer) {
		t.Fatalf("each role must be stronger than the one below: viewer %d, operator %d, cluster_admin %d", len(viewer), len(operator), len(clusterAdmin))
	}
}

func TestOperatorCannotSuppressDeleteOrChangeGlobalConfig(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleOperator)
	for _, forbidden := range []authorization.Permission{
		authorization.PermissionFindingsDelete,
		authorization.PermissionFindingsExceptionCreate,
		authorization.PermissionFindingsExceptionDelete,
		authorization.PermissionInventoryDelete,
		authorization.PermissionInvestigationsDelete,
		authorization.PermissionRulesWrite,
		authorization.PermissionRulesDelete,
		authorization.PermissionRulesImport,
		authorization.PermissionRuntimeMappingWrite,
		authorization.PermissionObservabilityLogsRead,
	} {
		if authorization.HasPermission(perms, forbidden) {
			t.Fatalf("operator must not have %s", forbidden)
		}
	}
}

func TestClusterAdminRunsItsClustersButNotThePlatform(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleClusterAdmin)
	for _, required := range []authorization.Permission{
		authorization.PermissionFindingsAck,
		authorization.PermissionFindingsDelete,
		authorization.PermissionFindingsExceptionCreate,
		authorization.PermissionFindingsExceptionDelete,
		authorization.PermissionInventoryDelete,
		authorization.PermissionInvestigationsDelete,
		authorization.PermissionRiskEvaluate,
	} {
		if !authorization.HasPermission(perms, required) {
			t.Fatalf("cluster_admin needs %s", required)
		}
	}
	// Rules, policies and mappings apply to every cluster, and error logs are not
	// cluster-scoped, so a role bound to some clusters must not hold them.
	for _, forbidden := range []authorization.Permission{
		authorization.PermissionAuthRegister,
		authorization.PermissionUsersRead,
		authorization.PermissionUsersRoleAssign,
		authorization.PermissionSystemAuditRead,
		authorization.PermissionObservabilityLogsRead,
		authorization.PermissionRulesWrite,
		authorization.PermissionRulesDelete,
		authorization.PermissionRulesImport,
		authorization.PermissionRuntimeMappingWrite,
		authorization.PermissionPoliciesDraft,
		authorization.PermissionPoliciesPublish,
		authorization.PermissionClusterCertificatesRotate,
	} {
		if authorization.HasPermission(perms, forbidden) {
			t.Fatalf("cluster_admin must not have %s", forbidden)
		}
	}
}

func TestFromStringsDropsUnknownPermissions(t *testing.T) {
	got := authorization.FromStrings([]string{"findings.read", "legacy.fake.permission", "  "})
	if len(got) != 1 || got[0] != authorization.PermissionFindingsRead {
		t.Fatalf("got %#v", got)
	}
}
