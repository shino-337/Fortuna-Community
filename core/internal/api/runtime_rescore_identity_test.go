package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRuntimeIngestTriggersScopedRescore(t *testing.T) {
	t.Setenv("RUNTIME_ATTACK_RESCORE_ENABLED", "true")
	t.Setenv("RUNTIME_ATTACK_RESCORE_DEBOUNCE", "1ms")
	t.Setenv("FORTUNA_RULES_DIR", t.TempDir())
	for _, version := range []string{"v2"} {
		t.Run(version, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			defer sqlDB.Close()
			require.NoError(t, db.AutoMigrate(&models.Pod{}, &models.Insight{}, &models.RiskScore{}, &models.RuntimeEvent{}, &models.RuntimeSignal{}, &models.RuntimeBehaviorFact{}, &models.RuntimeIncident{}))
			require.NoError(t, db.Exec("CREATE UNIQUE INDEX risk_identity ON risk_scores(resource_type,resource_uid,cluster_id)").Error)
			for _, cluster := range []string{"a", "b"} {
				require.NoError(t, db.Create(&models.Pod{ClusterID: cluster, UID: "same", Name: "p", Namespace: "ns"}).Error)
			}
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Request = c.Request.WithContext(resourceidentity.WithClusterID(c.Request.Context(), "a"))
			})
			handler := PostRuntimeEventsV2Scoped(db)
			r.POST("/events", handler)
			payload := `[{"source_record_id":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","pod":{"uid":"same","namespace":"ns"},"runtime":"falco","source":{"kind":"falco","rule":"test-rule"},"syscall":"open","target":"/tmp/ordinary","severity":"high","confidence":0.9}]`
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(payload))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			require.Eventually(t, func() bool {
				var n int64
				return db.Model(&models.RiskScore{}).Where("cluster_id = ? AND resource_uid = ?", "a", "same").Count(&n).Error == nil && n == 1
			}, 3*time.Second, 10*time.Millisecond)
			var foreign int64
			require.NoError(t, db.Model(&models.RiskScore{}).Where("cluster_id = ?", "b").Count(&foreign).Error)
			require.Zero(t, foreign)
		})
	}
}
