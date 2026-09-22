package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestScopedInventoryCollectionHTTPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []string{"empty", "failed", "invalid", "foreign", "legacy"} {
		t.Run(tc, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			pool, err := db.DB()
			require.NoError(t, err)
			pool.SetMaxOpenConns(1)
			require.NoError(t, db.AutoMigrate(&models.Agent{}, &models.Cluster{}, &models.InventoryCollection{}, &models.Pod{}, &models.AuditLog{}))
			r := gin.New()
			if tc != "legacy" {
				r.Use(func(c *gin.Context) {
					c.Set("fortuna.agent.principal", agentidentity.Principal{CredentialID: "c", ClusterID: "a", AgentID: "agent-a"})
					c.Next()
				})
			}
			r.POST("/sync", SyncDataFromAgent(db, nil))
			counts := map[string]int{}
			data := map[string]interface{}{"isFullSync": true, "isDeltaSync": false}
			for _, kind := range collection.InventoryKinds {
				counts[kind] = 0
				data[kind] = []interface{}{}
			}
			meta := collection.Inventory{Version: 1, ID: "http-attempt-00001", Status: "complete", StartedAt: time.Now().Add(-time.Second), ObservedAt: time.Now(), Counts: counts}
			cid := "a"
			want := http.StatusOK
			if tc == "failed" {
				meta.Status = "failed"
				data = map[string]interface{}{"isFullSync": false}
			}
			if tc == "invalid" {
				delete(data, "roles")
				want = http.StatusBadRequest
			}
			if tc == "foreign" {
				cid = "b"
				want = http.StatusForbidden
			}
			payload := map[string]interface{}{"clusterId": cid, "agent": map[string]interface{}{"agentId": "agent-a"}, "data": data, "collection": meta}
			raw, err := json.Marshal(payload)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/sync", bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, want, w.Code, w.Body.String())
			var receipts []models.InventoryCollection
			require.NoError(t, db.Find(&receipts).Error)
			if want != http.StatusOK {
				require.Empty(t, receipts)
				var n int64
				require.NoError(t, db.Model(&models.Agent{}).Count(&n).Error)
				require.Zero(t, n)
				return
			}
			require.Len(t, receipts, 1)
			require.Equal(t, "a", receipts[0].ClusterID)
			if tc == "legacy" {
				require.Equal(t, "unknown", receipts[0].Status)
			} else {
				require.Equal(t, meta.Status, receipts[0].Status)
				require.Equal(t, "agent-a", receipts[0].AgentID)
			}
			require.Contains(t, w.Body.String(), `"runtimeCoverage":"unknown"`)
		})
	}
}
