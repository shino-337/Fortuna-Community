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
	if got := authorization.NormalizeRole(models.RoleUserAdmin); got != models.RoleUserAdmin {
		t.Fatalf("got %q want %q", got, models.RoleUserAdmin)
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
	if authorization.HasPermission(perms, authorization.PermissionGraphQueryTraversal) {
		t.Fatal("viewer must not have graph.query.traversal")
	}
	if !authorization.HasPermission(perms, authorization.PermissionGraphReadSummary) {
		t.Fatal("viewer needs graph.read.summary")
	}
}

func TestOperatorCannotUploadMalware(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleOperator)
	if authorization.HasPermission(perms, authorization.PermissionMalwareUpload) {
		t.Fatal("operator must not have malware.upload")
	}
	if authorization.HasPermission(perms, authorization.PermissionAuthRegister) {
		t.Fatal("operator must not register users")
	}
}

func TestOperatorHasGraphTraversalNotAdvanced(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleOperator)
	if !authorization.HasPermission(perms, authorization.PermissionGraphQueryTraversal) {
		t.Fatal("operator needs graph.query.traversal")
	}
	if authorization.HasPermission(perms, authorization.PermissionGraphQueryAdvanced) {
		t.Fatal("operator must not have graph.query.advanced")
	}
}

func TestAdminHasAll(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleAdmin)
	all := authorization.AllPermissions()
	if len(perms) != len(all) {
		t.Fatalf("admin permission count %d != all %d", len(perms), len(all))
	}
}

func TestUserAdminMayManageUsersNotInventory(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleUserAdmin)
	if !authorization.HasPermission(perms, authorization.PermissionUsersUpdate) {
		t.Fatal("user_admin needs users.update")
	}
	if !authorization.HasPermission(perms, authorization.PermissionAuthRegister) {
		t.Fatal("user_admin needs auth.register")
	}
	if authorization.HasPermission(perms, authorization.PermissionFindingsRead) {
		t.Fatal("user_admin must not have findings.read")
	}
	if authorization.HasPermission(perms, authorization.PermissionFindingsAck) {
		t.Fatal("user_admin must not mutate findings")
	}
	if authorization.HasPermission(perms, authorization.PermissionInventoryRead) {
		t.Fatal("user_admin must not have inventory.read")
	}
}

func TestClusterAdminMayOperateScopedSecurityButNotPlatformAdmin(t *testing.T) {
	perms := authorization.PermissionsForRole(models.RoleClusterAdmin)
	for _, required := range []authorization.Permission{
		authorization.PermissionFindingsRead,
		authorization.PermissionFindingsAck,
		authorization.PermissionFindingsResolve,
		authorization.PermissionInventoryRead,
		authorization.PermissionRuntimeRead,
		authorization.PermissionGraphQueryTraversal,
		authorization.PermissionRiskEvaluate,
		authorization.PermissionObservabilityAgentsRead,
	} {
		if !authorization.HasPermission(perms, required) {
			t.Fatalf("cluster_admin needs %s", required)
		}
	}
	for _, forbidden := range []authorization.Permission{
		authorization.PermissionAuthRegister,
		authorization.PermissionUsersRead,
		authorization.PermissionUsersRoleAssign,
		authorization.PermissionSystemAuditRead,
		authorization.PermissionObservabilityLogsRead,
		authorization.PermissionRulesWrite,
		authorization.PermissionPoliciesPublish,
		authorization.PermissionMalwareUpload,
		authorization.PermissionFindingsDelete,
		authorization.PermissionFindingsExceptionDelete,
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

func TestMaxGraphTraversalDepth_AdvancedVsViewerCaps(t *testing.T) {
	adv := []authorization.Permission{authorization.PermissionGraphQueryAdvanced}
	if authorization.MaxGraphTraversalDepth(adv, 99) != 10 {
		t.Fatalf("advanced cap")
	}
	view := authorization.PermissionsForRole(models.RoleViewer)
	if authorization.MaxGraphTraversalDepth(view, 99) > 3 {
		t.Fatalf("viewer/path-only cap expected <=3 got %d", authorization.MaxGraphTraversalDepth(view, 99))
	}
}
