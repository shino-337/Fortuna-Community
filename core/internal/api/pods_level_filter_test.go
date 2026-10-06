package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fortuna/core/pkg/models"
)

func TestGetPodsRiskLevelFilterAndCounts(t *testing.T) {
	db := reviewDB(t, &models.Pod{}, &models.Insight{}, &models.Cluster{}, &models.RiskScore{})
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.Cluster{ID: "c1", Name: "c1"}).Error)
	for name, score := range map[string]float64{"crit": 82, "high": 55, "med": 30, "low": 10, "none": -1} {
		require.NoError(t, db.Create(&models.Pod{ClusterID: "c1", UID: "uid-" + name, Name: name, Namespace: "ns"}).Error)
		if score < 0 {
			continue
		}
		// An older, lower V3 row must not decide the level; the latest one does.
		require.NoError(t, db.Create(&models.RiskScore{ClusterID: "c1", ResourceType: "Pod", ResourceUID: "uid-" + name, TotalScore: 1, ScorerVersion: "v3", CalculatedAt: now.Add(-time.Hour)}).Error)
		require.NoError(t, db.Create(&models.RiskScore{ClusterID: "c1", ResourceType: "Pod", ResourceUID: "uid-" + name, TotalScore: score, ScorerVersion: "v3", CalculatedAt: now}).Error)
	}
	r := reviewRouter(adminUser())
	r.GET("/inventory/pods", GetPods(db))

	all := getJSON(t, r, "/inventory/pods?cluster=c1&withLevelCounts=1&level=high")
	require.Equal(t, float64(1), all["total"])
	require.Equal(t, "high", all["pods"].([]any)[0].(map[string]any)["name"])
	counts := all["levelCounts"].(map[string]any)
	require.Equal(t, map[string]any{"critical": float64(1), "high": float64(1), "medium": float64(1), "low": float64(1), "unscored": float64(1)}, counts)

	require.Equal(t, float64(1), getJSON(t, r, "/inventory/pods?cluster=c1&level=unscored")["total"])
	require.NotContains(t, getJSON(t, r, "/inventory/pods?cluster=c1"), "levelCounts")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/inventory/pods?level=severe", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}
