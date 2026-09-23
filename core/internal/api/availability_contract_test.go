package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func availabilityTestDB(t *testing.T, modelsToMigrate ...interface{}) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if len(modelsToMigrate) > 0 {
		require.NoError(t, db.AutoMigrate(modelsToMigrate...))
	}
	return db
}

func availabilityContext(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	return c, w
}

func decodeAvailabilityBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func TestAgentStatusMissingSchemaIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t)
	c, w := availabilityContext(http.MethodGet, "/api/v1/agents/status")
	GetAgentStatus(db)(c)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "unavailable", body["status"])
	require.Equal(t, "agent_status_schema_unavailable", body["code"])
	require.Equal(t, false, body["retryable"])
}

func TestAgentStatusUsesPersistedIdentityVersionAndHeartbeat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Cluster{}, &models.Agent{})
	require.NoError(t, db.Create(&models.Cluster{ID: "cluster-a", Name: "Cluster A"}).Error)
	require.NoError(t, db.Create(&models.Agent{
		ClusterID: "cluster-a",
		AgentID:   "agent-a",
		NodeName:  "node-a",
		Version:   "v9.9.9",
		Status:    "ready",
		LastSeenAt: nil,
	}).Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/agents/status")
	GetAgentStatus(db)(c)
	require.Equal(t, http.StatusOK, w.Code)

	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "available", body["dataStatus"])
	require.EqualValues(t, 0, body["healthy"])
	require.EqualValues(t, 1, body["disconnected"])
	agents := body["agents"].([]interface{})
	require.Len(t, agents, 1)
	agent := agents[0].(map[string]interface{})
	require.Equal(t, "cluster-a", agent["clusterId"])
	require.Equal(t, "Cluster A", agent["clusterName"])
	require.Equal(t, "v9.9.9", agent["version"])
	require.Equal(t, "disconnected", agent["status"])
	require.Nil(t, agent["lastHeartbeat"])
}

func TestSystemMetricsBackingQueryFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Cluster{})
	c, w := availabilityContext(http.MethodGet, "/api/v1/metrics/system")
	GetSystemMetrics(db)(c)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "system_metrics_pods_unavailable", body["code"])
}


func TestSystemMetricsCountsDuplicatePodUIDAcrossClustersSeparately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Cluster{}, &models.Pod{}, &models.ServiceAccount{}, &models.Insight{})
	now := time.Now().UTC()
	for _, clusterID := range []string{"cluster-a", "cluster-b"} {
		require.NoError(t, db.Create(&models.Cluster{ID: clusterID, Name: clusterID, LastSync: now}).Error)
		require.NoError(t, db.Create(&models.Pod{ClusterID: clusterID, UID: "same-pod", Name: "pod", Namespace: "ns"}).Error)
	}

	c, w := availabilityContext(http.MethodGet, "/api/v1/metrics/system")
	GetSystemMetrics(db)(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	resources := body["resources"].(map[string]interface{})
	require.EqualValues(t, 2, resources["pods"])
}

func TestDashboardIntegrityClusterQualifiesPodAndSBOMCoverage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Pod{}, &models.Insight{}, &models.SBOM{})
	for _, clusterID := range []string{"cluster-a", "cluster-b"} {
		require.NoError(t, db.Create(&models.Pod{ClusterID: clusterID, UID: "same-pod", Name: "pod", Namespace: "ns"}).Error)
	}
	require.NoError(t, db.Create(&models.SBOM{
		ClusterID: "cluster-a", PodUID: "same-pod", PodName: "pod", Namespace: "ns",
		ContainerName: "app", ImageName: "app", ImageTag: "1", Status: "complete",
	}).Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/health/dashboard-data-integrity")
	DashboardDataIntegrity(db)(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body DashboardDataIntegrityResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.EqualValues(t, 2, body.CrossChecks.PodsCount)
	require.EqualValues(t, 1, body.CrossChecks.PodsMissingSbom)
}

func TestClusterNodeSurfacesDoNotConvertMissingPodsTableToEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, handler := range map[string]func(*gorm.DB) gin.HandlerFunc{
		"nodes":     GetClusterNodes,
		"overview":  GetClusterOverview,
		"inventory": GetClusterInventory,
	} {
		t.Run(name, func(t *testing.T) {
			db := availabilityTestDB(t, &models.Cluster{})
			require.NoError(t, db.Create(&models.Cluster{ID: "cluster-a", Name: "Cluster A"}).Error)
			c, w := availabilityContext(http.MethodGet, "/clusters/cluster-a/"+name)
			c.Params = gin.Params{{Key: "id", Value: "cluster-a"}}
			handler(db)(c)
			require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
			body := decodeAvailabilityBody(t, w)
			require.Equal(t, "unavailable", body["status"])
		})
	}
}

func TestCapabilityDetailAndListShareUnavailableSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t)

	detailCtx, detailW := availabilityContext(http.MethodGet, "/pods/pod-a/capabilities")
	detailCtx.Params = gin.Params{{Key: "uid", Value: "pod-a"}}
	detailCtx.Set(middleware.CtxPodClusterID, "cluster-a")
	GetPodCapabilitiesScoped(db)(detailCtx)
	require.Equal(t, http.StatusServiceUnavailable, detailW.Code)
	detailBody := decodeAvailabilityBody(t, detailW)
	require.Equal(t, "capability_inventory_schema_unavailable", detailBody["code"])
	require.Equal(t, false, detailBody["retryable"])

	listCtx, listW := availabilityContext(http.MethodGet, "/capabilities")
	listCtx.Set("user", &models.User{Role: models.RoleAdmin})
	GetPodCapabilitiesList(db)(listCtx)
	require.Equal(t, http.StatusServiceUnavailable, listW.Code)
	listBody := decodeAvailabilityBody(t, listW)
	require.Equal(t, "capability_inventory_schema_unavailable", listBody["code"])
	require.Equal(t, false, listBody["retryable"])
}




func TestDashboardRuntimeHealthReadsPersistedTimestamps(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Pod{}, &models.Insight{}, &models.RuntimeEvent{}, &models.RuntimeSignal{})
	now := time.Now().UTC().Truncate(time.Microsecond)
	lastSeen := now.Format(time.RFC3339Nano)
	require.NoError(t, db.Create(&models.RuntimeEvent{
		ClusterID: "cluster-a", AgentID: "agent-a", PodUID: "pod-a", Namespace: "ns",
		Syscall: "execve", SourceKind: "falco", Runtime: "falco",
		ObservedAt: &now, IngestedAt: &now, CreatedAt: now,
	}).Error)
	require.NoError(t, db.Create(&models.RuntimeSignal{
		ClusterID: "cluster-a", PodUID: "pod-a", SignalType: "TEST", Category: "test",
		Confidence: 1, Evidence: "{}", LastSeenAt: &lastSeen, CreatedAt: now,
	}).Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/health/dashboard-data-integrity")
	DashboardDataIntegrity(db)(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body DashboardDataIntegrityResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.EqualValues(t, 1, body.RuntimeHealth.RuntimeEventsCount)
	require.EqualValues(t, 1, body.RuntimeHealth.RuntimeSignalsCount)
	require.EqualValues(t, 1, body.RuntimeHealth.FalcoEventsCount)
	require.NotNil(t, body.RuntimeHealth.LastRuntimeEventAt)
	require.NotNil(t, body.RuntimeHealth.LastRuntimeSignalAt)
}

func TestDashboardRuntimeHealthQueryFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Pod{}, &models.Insight{})
	require.NoError(t, db.Exec("CREATE TABLE runtime_events (id INTEGER PRIMARY KEY)").Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/health/dashboard-data-integrity")
	DashboardDataIntegrity(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "dashboard_runtime_health_unavailable", body["code"])
	require.Equal(t, true, body["retryable"])
}

func TestDashboardCatalogHealthQueryFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Pod{}, &models.Insight{})
	require.NoError(t, db.Exec("CREATE TABLE cves (id INTEGER PRIMARY KEY)").Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/health/dashboard-data-integrity")
	DashboardDataIntegrity(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "dashboard_catalog_health_unavailable", body["code"])
	require.Equal(t, true, body["retryable"])
}

func TestWorkerMetricsQueryFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t)
	require.NoError(t, db.Exec("CREATE TABLE sbom_match_runs (id INTEGER PRIMARY KEY)").Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/metrics/workers")
	GetWorkerStatus(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "worker_metrics_query_failed", body["code"])
}

func TestPolicyEvaluationMetricsFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t)
	c, w := availabilityContext(http.MethodGet, "/api/v1/metrics/policy-evaluation-cost")
	GetPolicyEvaluationCost(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "policy_evaluation_metrics_unavailable", body["code"])
}



func TestClusterAgentsMissingSchemaIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Cluster{})
	require.NoError(t, db.Create(&models.Cluster{ID: "cluster-a", Name: "Cluster A"}).Error)

	c, w := availabilityContext(http.MethodGet, "/clusters/cluster-a/agents")
	c.Params = gin.Params{{Key: "id", Value: "cluster-a"}}
	GetClusterAgents(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	require.Equal(t, "cluster_agents_schema_unavailable", decodeAvailabilityBody(t, w)["code"])
}

func TestClusterSecuritySummaryIsClusterQualified(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.PodCapability{})
	for _, clusterID := range []string{"cluster-a", "cluster-b"} {
		require.NoError(t, db.Create(&models.Cluster{ID: clusterID, Name: clusterID}).Error)
		require.NoError(t, db.Create(&models.Pod{
			ClusterID: clusterID, UID: "same-pod", Name: "pod", Namespace: "ns",
		}).Error)
	}
	require.NoError(t, db.Create(&models.Insight{
		ClusterID: "cluster-a", ResourceType: "Pod", ResourceNamespace: "ns",
		ResourceName: "pod", ResourceUID: "same-pod", InsightType: "runtime",
		Severity: "critical", Title: "a", Description: "a", Status: "active", DetectedAt: time.Now(),
	}).Error)
	require.NoError(t, db.Create(&models.Insight{
		ClusterID: "cluster-b", ResourceType: "Pod", ResourceNamespace: "ns",
		ResourceName: "pod", ResourceUID: "same-pod", InsightType: "runtime",
		Severity: "high", Title: "b", Description: "b", Status: "active", DetectedAt: time.Now(),
	}).Error)
	require.NoError(t, db.Create(&models.Insight{
		ClusterID: "cluster-a", ResourceType: "ServiceAccount", ResourceNamespace: "ns",
		ResourceName: "sa", ResourceUID: "same-pod", InsightType: "rbac",
		Severity: "high", Title: "non-pod", Description: "non-pod", Status: "active", DetectedAt: time.Now(),
	}).Error)
	require.NoError(t, db.Create(&models.PodCapability{
		ClusterID: "cluster-a", PodUID: "same-pod", Namespace: "ns",
		CapabilityID: "CAP_A", CapabilityGroup: "ESC", Severity: "critical",
	}).Error)
	require.NoError(t, db.Create(&models.PodCapability{
		ClusterID: "cluster-b", PodUID: "same-pod", Namespace: "ns",
		CapabilityID: "CAP_B", CapabilityGroup: "ESC", Severity: "high",
	}).Error)

	c, w := availabilityContext(http.MethodGet, "/clusters/cluster-a/security-summary")
	c.Params = gin.Params{{Key: "id", Value: "cluster-a"}}
	GetClusterSecuritySummary(db)(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.EqualValues(t, 1, body["criticalCount"])
	require.EqualValues(t, 0, body["highCount"])
	require.EqualValues(t, 1, body["capabilityCount"])
}

func TestClusterSecuritySummaryMissingCapabilitySchemaIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Cluster{}, &models.Pod{}, &models.Insight{})
	require.NoError(t, db.Create(&models.Cluster{ID: "cluster-a", Name: "Cluster A"}).Error)

	c, w := availabilityContext(http.MethodGet, "/clusters/cluster-a/security-summary")
	c.Params = gin.Params{{Key: "id", Value: "cluster-a"}}
	GetClusterSecuritySummary(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	require.Equal(t, "cluster_security_summary_capabilities_unavailable", decodeAvailabilityBody(t, w)["code"])
}


func TestPipelineHealthIsClusterScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t,
		&models.Pod{}, &models.PodCapability{}, &models.Insight{},
		&models.AttackPath{}, &models.RiskScore{},
	)
	now := time.Now().UTC()
	for _, clusterID := range []string{"sha256-a", "sha256-b"} {
		require.NoError(t, db.Create(&models.Pod{
			ClusterID: clusterID, UID: "same-pod", Name: "pod", Namespace: "ns",
		}).Error)
		require.NoError(t, db.Create(&models.PodCapability{
			ClusterID: clusterID, PodUID: "same-pod", Namespace: "ns",
			CapabilityID: "CAP_" + clusterID, CapabilityGroup: "ESC",
			Severity: "high", State: "exploited", UpdatedAt: now,
		}).Error)
		require.NoError(t, db.Create(&models.Insight{
			ClusterID: clusterID, ResourceType: "Pod", ResourceNamespace: "ns",
			ResourceName: "pod", ResourceUID: "same-pod", InsightType: "rbac",
			Severity: "high", Title: clusterID, Description: clusterID,
			Status: "active", DetectedAt: now, UpdatedAt: now,
		}).Error)
		require.NoError(t, db.Create(&models.AttackPath{
			ClusterID: clusterID, PodUID: "same-pod", PathID: "path-" + clusterID,
			TotalRisk: 10, UpdatedAt: now,
		}).Error)
		require.NoError(t, db.Create(&models.RiskScore{
			ClusterID: clusterID, ResourceType: "Pod", ResourceUID: "same-pod",
			ResourceName: "pod", Namespace: "ns", TotalScore: 80,
			ScorerVersion: "v3", CalculatedAt: now,
		}).Error)
	}

	c, w := availabilityContext(http.MethodGet, "/api/v1/monitoring/pipeline-health?cluster_id=sha256-a")
	c.Set("user", &models.User{Role: models.RoleAdmin})
	GetPipelineHealth(db)(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "available", body["dataStatus"])
	data := body["data"].(map[string]interface{})
	layer1 := data["layer1"].(map[string]interface{})
	layer2 := data["layer2"].(map[string]interface{})
	layer3 := data["layer3"].(map[string]interface{})
	layer4 := data["layer4"].(map[string]interface{})
	require.EqualValues(t, 1, layer1["insightCount"])
	require.EqualValues(t, 1, layer2["exploitedCapCount"])
	require.EqualValues(t, 1, layer3["totalPaths"])
	require.EqualValues(t, 1, layer3["criticalPaths"])
	require.EqualValues(t, 1, layer4["resourcesScored"])
	require.EqualValues(t, 1, layer4["v3Resources"])
}

func TestPipelineHealthQueryFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.PodCapability{})
	c, w := availabilityContext(http.MethodGet, "/api/v1/monitoring/pipeline-health")
	c.Set("user", &models.User{Role: models.RoleAdmin})
	GetPipelineHealth(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "pipeline_health_layer1_insights_unavailable", body["code"])
}


func TestPodListRiskCountsSeparateDuplicateUIDAcrossClusters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Pod{}, &models.Insight{})
	for _, clusterID := range []string{"cluster-a", "cluster-b"} {
		require.NoError(t, db.Create(&models.Pod{
			ClusterID: clusterID, UID: "same-pod", Name: "pod-" + clusterID, Namespace: "ns",
		}).Error)
	}
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Insight{
		ClusterID: "cluster-a", ResourceType: "Pod", ResourceName: "pod-a",
		ResourceUID: "same-pod", InsightType: "runtime", Severity: "high",
		Title: "a", Description: "a", Status: "active", DetectedAt: now,
	}).Error)
	for i := 0; i < 2; i++ {
		require.NoError(t, db.Create(&models.Insight{
			ClusterID: "cluster-b", ResourceType: "Pod", ResourceName: "pod-b",
			ResourceUID: "same-pod", InsightType: "runtime", Severity: "high",
			Title: "b", Description: "b", Status: "active", DetectedAt: now,
		}).Error)
	}
	require.NoError(t, db.Create(&models.Insight{
		ClusterID: "cluster-a", ResourceType: "ServiceAccount", ResourceName: "sa-a",
		ResourceUID: "same-pod", InsightType: "rbac", Severity: "critical",
		Title: "non-pod", Description: "must not contaminate pod risk count", Status: "active", DetectedAt: now,
	}).Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/pods?sortBy=risk_desc")
	c.Set("user", &models.User{Role: models.RoleAdmin})
	GetPods(db)(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	pods := body["pods"].([]interface{})
	require.Len(t, pods, 2)
	counts := map[string]int{}
	for _, item := range pods {
		pod := item.(map[string]interface{})
		counts[pod["clusterId"].(string)] = int(pod["riskCount"].(float64))
	}
	require.Equal(t, 1, counts["cluster-a"])
	require.Equal(t, 2, counts["cluster-b"])
}


func TestPodListRiskQueryFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Pod{})
	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a", UID: "pod-a", Name: "pod-a", Namespace: "ns",
	}).Error)
	// Table existence alone must not turn a broken risk projection into zero.
	require.NoError(t, db.Exec("CREATE TABLE insights (id INTEGER PRIMARY KEY, resource_uid TEXT)").Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/pods")
	c.Set("user", &models.User{Role: models.RoleAdmin})
	GetPods(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "pod_risk_counts_unavailable", body["code"])
	require.Equal(t, true, body["retryable"])
}


func TestPodListRiskScoreQueryFailureIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t, &models.Pod{}, &models.Insight{})
	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a", UID: "pod-a", Name: "pod-a", Namespace: "ns",
	}).Error)
	require.NoError(t, db.Exec("CREATE TABLE risk_scores (id INTEGER PRIMARY KEY, resource_uid TEXT)").Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/pods")
	c.Set("user", &models.User{Role: models.RoleAdmin})
	GetPods(db)(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "pod_risk_scores_unavailable", body["code"])
	require.Equal(t, true, body["retryable"])
}

func TestClusterStatsUsesPersistedAgentVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := availabilityTestDB(t,
		&models.Cluster{}, &models.Pod{}, &models.ServiceAccount{}, &models.Role{},
		&models.ClusterRole{}, &models.RoleBinding{}, &models.ClusterRoleBinding{},
		&models.Deployment{}, &models.Insight{}, &models.Agent{},
	)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Cluster{
		ID: "cluster-a", Name: "Cluster A", Source: "auto", LastSync: now,
	}).Error)
	require.NoError(t, db.Create(&models.Pod{
		ClusterID: "cluster-a", UID: "pod-a", Name: "pod-a", Namespace: "default",
	}).Error)
	require.NoError(t, db.Create(&models.Agent{
		ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-a",
		Version: "v2.7.4", Status: "ready", LastSeenAt: &now,
	}).Error)

	c, w := availabilityContext(http.MethodGet, "/api/v1/clusters/stats")
	GetClustersStats(db)(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeAvailabilityBody(t, w)
	require.Equal(t, "available", body["dataStatus"])
	clusters := body["clusters"].([]interface{})
	require.Len(t, clusters, 1)
	cluster := clusters[0].(map[string]interface{})
	require.Equal(t, "v2.7.4", cluster["agentVersion"])
	require.NotEqual(t, "v1.0.0", cluster["agentVersion"])
}
