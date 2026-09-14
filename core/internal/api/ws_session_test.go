package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func wsSessionFixture(t *testing.T) (*gorm.DB, models.User, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(&models.User{}, &models.UserSession{}, &models.Pod{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "ws-viewer", Email: "ws@test.local", Role: models.RoleViewer, Active: true, ScopeJSON: `{"cluster_ids":["a"]}`}
	if err = db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.Pod{UID: "pod-a", ClusterID: "a", Name: "a", Namespace: "default"}).Error; err != nil {
		t.Fatal(err)
	}
	sid, err := sessions.CreateLoginSession(db, user.ID, 1, "test", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	return db, user, sid
}

func TestWSAuthorizationReloadsServerState(t *testing.T) {
	for _, name := range []string{"revoked", "session expired", "inactive", "password change required", "password changed", "role removed", "scope removed", "JWT expired", "database failure"} {
		t.Run(name, func(t *testing.T) {
			db, u, sid := wsSessionFixture(t)
			g := &wsAuthorization{db: db, ctx: context.Background(), userID: u.ID, sessionID: sid, requireSession: true, expiresAt: time.Now().Add(time.Hour), permission: authorization.PermissionInventoryRead, podUID: "pod-a"}
			if err := g.validate(); err != nil {
				t.Fatal(err)
			}
			var err error
			switch name {
			case "revoked":
				err = db.Model(&models.UserSession{}).Where("id = ?", sid).Update("revoked_at", time.Now()).Error
			case "session expired":
				err = db.Model(&models.UserSession{}).Where("id = ?", sid).Update("expires_at", time.Now().Add(-time.Second)).Error
			case "inactive":
				err = db.Model(&u).Update("active", false).Error
			case "password change required":
				err = db.Model(&u).Update("must_change_password", true).Error
			case "password changed":
				err = db.Model(&u).Update("password_changed_at", time.Now().Add(time.Second)).Error
			case "role removed":
				err = db.Model(&u).Update("role", models.RoleUserAdmin).Error
			case "scope removed":
				err = db.Model(&u).Update("scope_json", `{"cluster_ids":["b"]}`).Error
			case "JWT expired":
				g.expiresAt = time.Now().Add(-time.Second)
			case "database failure":
				err = db.Migrator().DropTable(&models.User{})
			}
			if err != nil {
				t.Fatal(err)
			}
			if g.validate() == nil {
				t.Fatal("revoked authorization remained valid")
			}
		})
	}
}

func TestWebSocketHandlersRejectQueuedDataAfterLogout(t *testing.T) {
	for _, pod := range []bool{false, true} {
		t.Run(map[bool]string{true: "pod", false: "risks"}[pod], func(t *testing.T) {
			db, u, sid := wsSessionFixture(t)
			const secret = "websocket-lifecycle-test-secret"
			token, err := auth.GenerateToken(u.ID, u.Username, u.Role, nil, sid, secret, 1)
			if err != nil {
				t.Fatal(err)
			}
			r := gin.New()
			r.Use(middleware.AuthMiddleware(db, secret))
			path := "/ws/risks"
			if pod {
				path = "/ws/pod/pod-a"
				r.GET("/ws/pod/:uid", middleware.RequirePodUIDClusterScope(db, "uid"), PodDetailWS(db))
			} else {
				r.GET(path, RisksWS(db))
			}
			srv := httptest.NewServer(r)
			defer srv.Close()
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, http.Header{"Authorization": []string{"Bearer " + token}})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err = db.Model(&models.UserSession{}).Where("id = ?", sid).Update("revoked_at", time.Now()).Error; err != nil {
				t.Fatal(err)
			}
			// The server may still be registering after the handshake. Repeat notifications
			// until the first queued write rechecks the revoked session.
			stop := make(chan struct{})
			stopped := make(chan struct{})
			defer func() { close(stop); <-stopped }()
			go func() {
				defer close(stopped)
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				for {
					select {
					case <-stop:
						return
					case <-ticker.C:
						if pod {
							BroadcastPodDetailUpdate("pod-a", "events")
						} else {
							BroadcastRisksUpdate()
						}
					}
				}
			}()
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, data, err := conn.ReadMessage()
			if !websocket.IsCloseError(err, websocket.ClosePolicyViolation) {
				t.Fatalf("wanted policy close, got data=%s err=%v", data, err)
			}
		})
	}
}

func TestIdleWebSocketRevocationAndTokenDeadline(t *testing.T) {
	for _, expire := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle revocation", true: "JWT deadline"}[expire], func(t *testing.T) {
			db, u, sid := wsSessionFixture(t)
			ready := make(chan struct{})
			r := gin.New()
			r.GET("/ws", func(c *gin.Context) {
				conn, err := (&websocket.Upgrader{}).Upgrade(c.Writer, c.Request, nil)
				if err != nil {
					return
				}
				expiry := time.Now().Add(time.Hour)
				interval := 50 * time.Millisecond
				if expire {
					expiry = time.Now().Add(150 * time.Millisecond)
					interval = time.Second
				}
				guard := &wsAuthorization{db: db, ctx: c.Request.Context(), userID: u.ID, sessionID: sid, requireSession: true, expiresAt: expiry, permission: authorization.PermissionFindingsRead}
				close(ready)
				serveAuthorizedWebSocket(conn, make(chan []byte), guard, interval)
			})
			srv := httptest.NewServer(r)
			defer srv.Close()
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			<-ready
			if !expire {
				if err = db.Model(&models.UserSession{}).Where("id = ?", sid).Update("revoked_at", time.Now()).Error; err != nil {
					t.Fatal(err)
				}
			}
			_ = conn.SetReadDeadline(time.Now().Add(time.Second))
			_, _, err = conn.ReadMessage()
			if !websocket.IsCloseError(err, websocket.ClosePolicyViolation) {
				t.Fatalf("wanted idle policy close, got %v", err)
			}
		})
	}
}
