package migrations

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/policy"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBaselinePodPoliciesCompileAndDetectUnsafeSpec(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:baseline_pod_policy?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, Migration096_PolicyEngineBaselineBootstrap(db))

	evaluator, err := policy.NewEvaluator(db)
	require.NoError(t, err)
	defer evaluator.Shutdown()

	base := func() map[string]interface{} {
		return map[string]interface{}{
			"containers": []interface{}{map[string]interface{}{
				"name": "app", "securityContext": map[string]interface{}{"privileged": false},
			}},
		}
	}
	tests := []struct {
		name string
		edit func(map[string]interface{})
		want string
	}{
		{name: "compliant", edit: func(map[string]interface{}) {}},
		{name: "privileged container", edit: func(spec map[string]interface{}) {
			spec["containers"] = []interface{}{map[string]interface{}{"securityContext": map[string]interface{}{"privileged": true}}}
		}, want: "k8s-no-privileged-container"},
		{name: "privileged init container", edit: func(spec map[string]interface{}) {
			spec["initContainers"] = []interface{}{map[string]interface{}{"securityContext": map[string]interface{}{"privileged": true}}}
		}, want: "k8s-no-privileged-container"},
		{name: "privileged ephemeral container", edit: func(spec map[string]interface{}) {
			spec["ephemeralContainers"] = []interface{}{map[string]interface{}{"securityContext": map[string]interface{}{"privileged": true}}}
		}, want: "k8s-no-privileged-container"},
		{name: "host network", edit: func(spec map[string]interface{}) { spec["hostNetwork"] = true }, want: "k8s-no-host-namespace-sharing"},
		{name: "host PID", edit: func(spec map[string]interface{}) { spec["hostPID"] = true }, want: "k8s-no-host-namespace-sharing"},
		{name: "host IPC", edit: func(spec map[string]interface{}) { spec["hostIPC"] = true }, want: "k8s-no-host-namespace-sharing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := base()
			tt.edit(spec)
			violations, err := evaluator.EvaluateFast(context.Background(), &policy.Resource{
				Type: "Pod", UID: "pod-1", Name: "pod", Namespace: "default", ClusterID: "cluster-1", Spec: spec,
			})
			require.NoError(t, err)
			if tt.want == "" {
				require.Empty(t, violations)
				return
			}
			require.Len(t, violations, 1)
			require.Equal(t, tt.want, violations[0].TemplateID)
		})
	}
}

func TestMigration151RepointsOnlyStockBrokenPolicyVersions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "policy.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PolicyTemplate{}, &models.PolicyInstance{}))

	stock := []struct{ id, expression string }{
		{"k8s-no-privileged-container", legacyNoPrivilegedCEL},
		{"k8s-no-host-namespace-sharing", legacyNoHostNamespacesCEL},
	}
	for _, item := range stock {
		require.NoError(t, db.Create(&models.PolicyTemplate{
			TemplateID: item.id, Version: "1.0.0", Name: item.id, Category: "security",
			DefaultSeverity: "high", CELExpression: item.expression, CreatedBy: "system", IsSystem: true,
		}).Error)
		require.NoError(t, db.Create(&models.PolicyInstance{
			TemplateID: item.id, TemplateVersion: "1.0.0", InstanceName: "custom-" + item.id,
			Enabled: true, Action: "alert", Severity: "high",
		}).Error)
	}
	require.NoError(t, db.Create(&models.PolicyInstance{
		TemplateID: "k8s-no-privileged-container", TemplateVersion: "1.0.0",
		InstanceName: "baseline-no-privileged-container", Enabled: true, Action: "alert", Severity: "high",
	}).Error)

	for i := 0; i < 2; i++ {
		require.NoError(t, Migration151_RepairBaselinePodPolicyCEL(db))
	}
	for _, item := range stock {
		var old, corrected models.PolicyTemplate
		require.NoError(t, db.Unscoped().Where("template_id = ? AND version = ?", item.id, "1.0.0").First(&old).Error)
		require.True(t, old.DeletedAt.Valid)
		require.NoError(t, db.Where("template_id = ? AND version = ?", item.id, baselinePodPolicyVersion).First(&corrected).Error)
		require.NotEqual(t, item.expression, corrected.CELExpression)
		var instance models.PolicyInstance
		require.NoError(t, db.Where("instance_name = ?", "custom-"+item.id).First(&instance).Error)
		require.Equal(t, baselinePodPolicyVersion, instance.TemplateVersion)
	}
	var baseline models.PolicyInstance
	require.NoError(t, db.Where("instance_name = ?", "baseline-no-privileged-container").First(&baseline).Error)
	require.Equal(t, models.StringArray{"Pod"}, baseline.ResourceTypes)
}

func TestMigration151PreservesModifiedLegacyTemplate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "policy.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PolicyTemplate{}, &models.PolicyInstance{}))
	require.NoError(t, seedBaselinePolicyTemplates(db))
	require.NoError(t, db.Create(&models.PolicyTemplate{
		TemplateID: "k8s-no-privileged-container", Version: "1.0.0", Name: "modified",
		Category: "security", DefaultSeverity: "high", CELExpression: "true",
		CreatedBy: "system", IsSystem: true,
	}).Error)
	require.NoError(t, db.Create(&models.PolicyInstance{
		TemplateID: "k8s-no-privileged-container", TemplateVersion: "1.0.0",
		InstanceName: "custom-modified", Enabled: true, Action: "alert", Severity: "high",
	}).Error)

	require.NoError(t, Migration151_RepairBaselinePodPolicyCEL(db))
	var old models.PolicyTemplate
	require.NoError(t, db.Where("template_id = ? AND version = ?", "k8s-no-privileged-container", "1.0.0").First(&old).Error)
	require.Equal(t, "true", old.CELExpression)
	var instance models.PolicyInstance
	require.NoError(t, db.Where("instance_name = ?", "custom-modified").First(&instance).Error)
	require.Equal(t, "1.0.0", instance.TemplateVersion)
}

func TestMigration151RepointsOrphanedLegacyBaselineInstances(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "policy.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.PolicyTemplate{}, &models.PolicyInstance{}))
	require.NoError(t, seedBaselinePolicyTemplates(db)) // Recreated 1.0.1 templates; 1.0.0 rows are missing.
	for _, item := range []struct{ id, name string }{
		{"k8s-no-privileged-container", "baseline-no-privileged-container"},
		{"k8s-no-host-namespace-sharing", "baseline-no-host-namespace-sharing"},
	} {
		require.NoError(t, db.Create(&models.PolicyInstance{
			TemplateID: item.id, TemplateVersion: "1.0.0", InstanceName: item.name,
			Enabled: true, Action: "alert", Severity: "high",
		}).Error)
	}
	require.NoError(t, Migration151_RepairBaselinePodPolicyCEL(db))
	var instances []models.PolicyInstance
	require.NoError(t, db.Find(&instances).Error)
	require.Len(t, instances, 2)
	for _, instance := range instances {
		require.Equal(t, baselinePodPolicyVersion, instance.TemplateVersion)
		require.Equal(t, models.StringArray{"Pod"}, instance.ResourceTypes)
	}
	evaluator, err := policy.NewEvaluator(db)
	require.NoError(t, err)
	defer evaluator.Shutdown()
	violations, err := evaluator.EvaluateFast(context.Background(), &policy.Resource{
		Type: "Pod", Namespace: "default", ClusterID: "cluster-1",
		Spec: map[string]interface{}{
			"containers":  []interface{}{map[string]interface{}{"securityContext": map[string]interface{}{"privileged": true}}},
			"hostNetwork": true,
		},
	})
	require.NoError(t, err)
	require.Len(t, violations, 2)
}
