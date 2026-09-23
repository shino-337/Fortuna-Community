package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
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
	require.Equal(t, true, body["retryable"])
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
	require.Equal(t, "capability_inventory_schema_unavailable", decodeAvailabilityBody(t, detailW)["code"])

	listCtx, listW := availabilityContext(http.MethodGet, "/capabilities")
	listCtx.Set("user", &models.User{Role: models.RoleAdmin})
	GetPodCapabilitiesList(db)(listCtx)
	require.Equal(t, http.StatusServiceUnavailable, listW.Code)
	require.Equal(t, "capability_inventory_schema_unavailable", decodeAvailabilityBody(t, listW)["code"])
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
