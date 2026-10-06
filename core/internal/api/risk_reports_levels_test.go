package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

// Pod detail lists the pod's findings from this report; each needs the same risk level as on Findings.
func TestGetPodRiskReportCarriesRiskLevels(t *testing.T) {
	db := reviewDB(t, &models.ServiceAccount{}, &models.Role{}, &models.ClusterRole{}, &models.RoleBinding{}, &models.ClusterRoleBinding{},
		&models.Cluster{}, &models.Pod{}, &models.Insight{}, &models.RuntimeSignal{}, &models.RiskScore{})
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Cluster{ID: "c1", Name: "c1"}).Error)
	require.NoError(t, db.Create(&models.Pod{ClusterID: "c1", UID: "pod-1", Name: "app", Namespace: "ns", ServiceAccount: "default"}).Error)
	ins := models.Insight{ClusterID: "c1", ResourceType: "Pod", ResourceNamespace: "ns", ResourceName: "app", ResourceUID: "pod-1",
		InsightType: "misconfiguration", Severity: "critical", Title: "t", Description: "d", Status: "active", DetectedAt: now}
	require.NoError(t, db.Create(&ins).Error)
	require.NoError(t, db.Create(&models.RiskScore{ClusterID: "c1", ResourceType: "Pod", ResourceUID: "pod-1", TotalScore: 55, ScorerVersion: "v3", CalculatedAt: now}).Error)

	r := gin.New()
	useAdminTestPrincipal(r)
	r.GET("/risk/pods/:uid/report", middleware.RequirePodUIDClusterScope(db, "uid"), GetPodRiskReport(db))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/risk/pods/pod-1/report", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		InsightLevels map[string]string `json:"insightLevels"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	// Score 55 is high; the rule severity (critical) is only a hint.
	require.Equal(t, "high", body.InsightLevels[strconv.FormatUint(uint64(ins.ID), 10)])
}
