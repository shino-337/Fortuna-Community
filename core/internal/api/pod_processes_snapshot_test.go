package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

func TestPodProcessesServeOnlyLatestSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Pod{}, &models.PodProcess{}))
	require.NoError(t, db.Create(&models.Pod{UID: "pod-a", Name: "a", Namespace: "ns", ClusterID: "c1"}).Error)
	require.NoError(t, db.Create(&models.Pod{UID: "pod-a", Name: "a", Namespace: "ns", ClusterID: "c2"}).Error)

	older := time.Now().UTC().Add(-4 * time.Minute).Truncate(time.Second)
	newer := older.Add(2 * time.Minute)
	for _, p := range []models.PodProcess{
		{ClusterID: "c1", PodUID: "pod-a", Namespace: "ns", ContainerName: "app", PID: 1, Command: "init", ObservedAt: older},
		{ClusterID: "c1", PodUID: "pod-a", Namespace: "ns", ContainerName: "app", PID: 7, Command: "old-worker", ObservedAt: older},
		{ClusterID: "c1", PodUID: "pod-a", Namespace: "ns", ContainerName: "app", PID: 1, Command: "init", ObservedAt: newer},
		{ClusterID: "c1", PodUID: "pod-a", Namespace: "ns", ContainerName: "app", PID: 9, Command: "new-worker", ObservedAt: newer},
		// Same Pod UID in another cluster must not leak into the snapshot or move "latest".
		{ClusterID: "c2", PodUID: "pod-a", Namespace: "ns", ContainerName: "app", PID: 3, Command: "foreign", ObservedAt: newer.Add(time.Minute)},
	} {
		require.NoError(t, db.Create(&p).Error)
	}

	r := gin.New()
	useAdminTestPrincipal(r)
	r.GET("/runtime/pods/:uid/processes", middleware.RequirePodUIDClusterScope(db, "uid"), GetPodProcessesByUIDScoped(db))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/runtime/pods/pod-a/processes?clusterId=c1", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body struct {
		ClusterID string              `json:"clusterId"`
		Items     []models.PodProcess `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "c1", body.ClusterID)
	require.Len(t, body.Items, 2)
	require.Equal(t, []int{1, 9}, []int{body.Items[0].PID, body.Items[1].PID})
	for _, item := range body.Items {
		require.True(t, item.ObservedAt.Equal(newer), item.ObservedAt)
	}
}
