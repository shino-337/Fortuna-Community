package migrations

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/policy"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestBaselinePodPolicySeedAndRepairPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	schema := fmt.Sprintf("policy_baseline_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	require.NoError(t, db.AutoMigrate(&models.PolicyTemplate{}, &models.PolicyInstance{}))

	for i := 0; i < 2; i++ {
		require.NoError(t, Migration145_SeedPolicyEngineBaselineSQL(db))
	}
	var templates []models.PolicyTemplate
	require.NoError(t, db.Order("template_id").Find(&templates).Error)
	require.Len(t, templates, 2)
	for _, template := range templates {
		require.Equal(t, baselinePodPolicyVersion, template.Version)
	}
	var instances []models.PolicyInstance
	require.NoError(t, db.Order("instance_name").Find(&instances).Error)
	require.Len(t, instances, 2)
	for _, instance := range instances {
		require.Equal(t, baselinePodPolicyVersion, instance.TemplateVersion)
		require.Equal(t, models.StringArray{"Pod"}, instance.ResourceTypes)
	}
	evaluator, err := policy.NewEvaluator(db)
	require.NoError(t, err)
	violations, err := evaluator.EvaluateFast(context.Background(), &policy.Resource{
		Type: "Pod", UID: "pod-1", Namespace: "default", ClusterID: "cluster-1",
		Spec: map[string]interface{}{"containers": []interface{}{map[string]interface{}{
			"securityContext": map[string]interface{}{"privileged": true},
		}}},
	})
	evaluator.Shutdown()
	require.NoError(t, err)
	require.Len(t, violations, 1)
	require.Equal(t, "k8s-no-privileged-container", violations[0].TemplateID)

	// Existing databases retain 1.0.0 system rows. The forward repair must
	// repoint all consumers without changing custom instance actions or scopes.
	require.NoError(t, db.Create(&models.PolicyTemplate{
		TemplateID: "k8s-no-privileged-container", Version: "1.0.0", Name: "old",
		Category: "security", DefaultSeverity: "high", CELExpression: legacyNoPrivilegedCEL,
		DefaultScope: "{}", RemediationTemplate: "{}", Examples: "{}",
		CreatedBy: "system", IsSystem: true,
	}).Error)
	require.NoError(t, db.Create(&models.PolicyInstance{
		TemplateID: "k8s-no-privileged-container", TemplateVersion: "1.0.0",
		InstanceName: "custom-legacy", Enabled: true, Action: "block", Severity: "critical",
		Namespaces: models.StringArray{"production"}, LabelSelectors: "{}", Exemptions: "{}",
	}).Error)
	require.NoError(t, Migration151_RepairBaselinePodPolicyCEL(db))
	var old models.PolicyTemplate
	require.NoError(t, db.Unscoped().Where("template_id = ? AND version = ?", "k8s-no-privileged-container", "1.0.0").First(&old).Error)
	require.True(t, old.DeletedAt.Valid)
	var custom models.PolicyInstance
	require.NoError(t, db.Where("instance_name = ?", "custom-legacy").First(&custom).Error)
	require.Equal(t, baselinePodPolicyVersion, custom.TemplateVersion)
	require.Equal(t, "block", custom.Action)
	require.Equal(t, "critical", custom.Severity)
	require.Equal(t, models.StringArray{"production"}, custom.Namespaces)
}
