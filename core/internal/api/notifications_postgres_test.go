package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/models"
)

func notificationsPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = adminSQL.Close() })

	schema := fmt.Sprintf("notification_reads_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() { _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error })

	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })
	return db
}

// The read-state migration and the handlers' INSERT ... SELECT run against
// PostgreSQL, which types bind parameters more strictly than SQLite.
func TestNotificationReadsPostgres(t *testing.T) {
	db := notificationsPostgresDB(t)
	require.NoError(t, migrations.Migration056_AddNotificationsTable(db))
	require.NoError(t, migrations.Migration148_NotificationsContextFields(db))
	require.NoError(t, migrations.Migration149_NotificationsResourceName(db))
	require.NoError(t, db.Exec(`CREATE TABLE users (id SERIAL PRIMARY KEY, deleted_at TIMESTAMPTZ)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id) VALUES (1), (2)`).Error)

	readAt := time.Now().UTC().Add(-time.Hour)
	old := models.Notification{Title: "already read", ClusterID: "c1", ReadAt: &readAt}
	fresh := models.Notification{Title: "new", ClusterID: "c1"}
	require.NoError(t, db.Create(&old).Error)
	require.NoError(t, db.Create(&fresh).Error)

	require.NoError(t, migrations.Migration152_NotificationReads(db))
	require.NoError(t, migrations.Migration152_NotificationReads(db), "migration must be re-runnable")
	var backfilled int64
	require.NoError(t, db.Model(&models.NotificationRead{}).Where("notification_id = ?", old.ID).Count(&backfilled).Error)
	require.EqualValues(t, 2, backfilled, "a notification read before the upgrade stays read for every user")

	r := reviewRouter(&models.User{ID: 1, Role: models.RoleAdmin, Active: true})
	r.GET("/notifications", GetNotifications(db))
	r.PATCH("/notifications/:id/read", MarkNotificationRead(db))
	r.POST("/notifications/read-all", MarkAllNotificationsRead(db))
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/notifications?unreadOnly=true", nil),
		httptest.NewRequest(http.MethodPatch, "/notifications/"+strconv.Itoa(int(fresh.ID))+"/read", nil),
		httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil),
		httptest.NewRequest(http.MethodGet, "/notifications", nil),
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code, req.URL.String()+": "+w.Body.String())
	}

	var user1, user2 int64
	require.NoError(t, db.Model(&models.NotificationRead{}).Where("user_id = 1").Count(&user1).Error)
	require.NoError(t, db.Model(&models.NotificationRead{}).Where("user_id = 2").Count(&user2).Error)
	require.EqualValues(t, 2, user1)
	require.EqualValues(t, 1, user2, "user 1 reading must not mark it read for user 2")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.JSONEq(t, `{"updated":0}`, w.Body.String())
}
