package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func runtimeAckFixture(t *testing.T) (*gorm.DB, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&models.Pod{}, &models.RuntimeEvent{}, &models.RuntimeEventIngestClaim{},
		&models.RuntimeSignal{}, &models.PodRiskProfile{},
	))
	require.NoError(t, db.Create(&models.Pod{UID: "pod-ack", ClusterID: "cluster-a", Namespace: "ns", Name: "pod"}).Error)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(resourceidentity.WithClusterID(c.Request.Context(), "cluster-a"))
		c.Next()
	})
	r.POST("/events", PostRuntimeEventsV2Scoped(db))
	return db, r
}

func runtimeAckPayload(eventID, target string) map[string]interface{} {
	return map[string]interface{}{
		"event_id": eventID,
		"pod": map[string]interface{}{"uid": "pod-ack", "namespace": "ns", "name": "pod"},
		"syscall": "read", "target": target, "confidence": 0.5,
		"source_kind": "agent", "source_sensor_id": "sensor-a",
	}
}

func postRuntimeAck(t *testing.T, r http.Handler, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRuntimeEventBatchACKIsIdempotentAndImmutable(t *testing.T) {
	db, r := runtimeAckFixture(t)
	payload := []map[string]interface{}{runtimeAckPayload("ack-event-0001", "/tmp/a")}

	first := postRuntimeAck(t, r, payload)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Contains(t, first.Body.String(), `"accepted":1`)
	require.Contains(t, first.Body.String(), `"replayed":0`)

	second := postRuntimeAck(t, r, payload)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Contains(t, second.Body.String(), `"accepted":1`)
	require.Contains(t, second.Body.String(), `"replayed":1`)

	altered := []map[string]interface{}{runtimeAckPayload("ack-event-0001", "/tmp/changed")}
	conflict := postRuntimeAck(t, r, altered)
	require.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())

	var events, claims int64
	require.NoError(t, db.Model(&models.RuntimeEvent{}).Count(&events).Error)
	require.NoError(t, db.Model(&models.RuntimeEventIngestClaim{}).Count(&claims).Error)
	require.EqualValues(t, 1, events)
	require.EqualValues(t, 1, claims)
}

func TestRuntimeEventBatchValidationHasNoPartialEffects(t *testing.T) {
	db, r := runtimeAckFixture(t)
	bad := runtimeAckPayload("ack-event-0002", "/tmp/b")
	delete(bad, "confidence")
	payload := []map[string]interface{}{runtimeAckPayload("ack-event-0001", "/tmp/a"), bad}

	w := postRuntimeAck(t, r, payload)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	var events, claims int64
	require.NoError(t, db.Model(&models.RuntimeEvent{}).Count(&events).Error)
	require.NoError(t, db.Model(&models.RuntimeEventIngestClaim{}).Count(&claims).Error)
	require.Zero(t, events)
	require.Zero(t, claims)
}

func TestRuntimeEventBatchPersistenceFailureRollsBackWholeBatch(t *testing.T) {
	db, r := runtimeAckFixture(t)
	writes := 0
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:runtime-event-second-write-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "runtime_events" {
			writes++
			if writes == 2 {
				tx.AddError(errors.New("injected runtime persistence failure"))
			}
		}
	}))
	defer db.Callback().Create().Remove("test:runtime-event-second-write-failure")

	payload := []map[string]interface{}{
		runtimeAckPayload("ack-event-0001", "/tmp/a"),
		runtimeAckPayload("ack-event-0002", "/tmp/b"),
	}
	w := postRuntimeAck(t, r, payload)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

	var events, claims int64
	require.NoError(t, db.Model(&models.RuntimeEvent{}).Count(&events).Error)
	require.NoError(t, db.Model(&models.RuntimeEventIngestClaim{}).Count(&claims).Error)
	require.Zero(t, events, "failed batch must roll back earlier raw events")
	require.Zero(t, claims, "failed batch must roll back ingest claims")
}

func TestRuntimeEventBatchRejectsDuplicateEventIDBeforeEffects(t *testing.T) {
	db, r := runtimeAckFixture(t)
	payload := []map[string]interface{}{
		runtimeAckPayload("ack-event-same", "/tmp/a"),
		runtimeAckPayload("ack-event-same", "/tmp/a"),
	}
	w := postRuntimeAck(t, r, payload)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	var events int64
	require.NoError(t, db.Model(&models.RuntimeEvent{}).Count(&events).Error)
	require.Zero(t, events)
}
