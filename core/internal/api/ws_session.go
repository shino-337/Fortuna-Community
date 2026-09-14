package api

import (
	"context"
	"errors"
	"time"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

const wsAuthorizationInterval = 15 * time.Second

var errWSAuthorization = errors.New("websocket authorization no longer valid")

type wsAuthorization struct {
	db             *gorm.DB
	ctx            context.Context
	userID         uint
	sessionID      string
	requireSession bool
	expiresAt      time.Time
	permission     authorization.Permission
	podUID         string
	development    bool
}

func newWSAuthorization(db *gorm.DB, c *gin.Context, permission authorization.Permission, podUID string) (*wsAuthorization, error) {
	user, ok := c.Get("user")
	if !ok {
		return nil, errWSAuthorization
	}
	u, ok := user.(*models.User)
	if !ok || u == nil {
		return nil, errWSAuthorization
	}
	g := &wsAuthorization{db: db, ctx: c.Request.Context(), userID: u.ID, sessionID: c.GetString(middleware.CtxJWTSessionID), permission: permission, podUID: podUID}
	if c.GetString("auth_source") == "dev_principal" && u.ID == 0 {
		g.development = authorization.HasPermission(middleware.GrantedPermissions(c), permission)
		if !g.development {
			return nil, errWSAuthorization
		}
		return g, nil
	}
	expiry, ok := c.Get(middleware.CtxJWTExpiresAt)
	if !ok {
		return nil, errWSAuthorization
	}
	g.expiresAt, ok = expiry.(time.Time)
	if !ok || g.expiresAt.IsZero() {
		return nil, errWSAuthorization
	}
	g.requireSession = sessions.TableExists(db)
	return g, g.validate()
}

// Never retain a mutable Gin context or the handshake's role/scope snapshot.
func (g *wsAuthorization) validate() error {
	if g.development {
		return nil
	}
	if g.db == nil || !time.Now().Before(g.expiresAt) {
		return errWSAuthorization
	}
	ctx, cancel := context.WithTimeout(g.ctx, 3*time.Second)
	defer cancel()
	db := g.db.WithContext(ctx)
	var user models.User
	if err := db.First(&user, g.userID).Error; err != nil {
		return errWSAuthorization
	}
	if !user.Active || user.MustChangePassword || !authorization.HasPermission(authorization.PermissionsForUser(user.Role), g.permission) {
		return errWSAuthorization
	}
	if g.requireSession {
		var session models.UserSession
		if g.sessionID == "" || db.Where("id = ? AND user_id = ?", g.sessionID, g.userID).First(&session).Error != nil {
			return errWSAuthorization
		}
		if session.RevokedAt != nil || !time.Now().Before(session.ExpiresAt) || (user.PasswordChangedAt != nil && session.IssuedAt.Before(*user.PasswordChangedAt)) {
			return errWSAuthorization
		}
	}
	if g.podUID != "" && authorization.NormalizeRole(user.Role) != models.RoleAdmin {
		scope := authorization.ParseScopeDocument(user.ScopeJSON)
		if scope.RestrictsClusters() {
			var pod models.Pod
			if db.Unscoped().Select("cluster_id").Where("uid = ?", g.podUID).First(&pod).Error != nil || pod.ClusterID == "" || !scope.ClusterAllowed(pod.ClusterID) {
				return errWSAuthorization
			}
		}
	}
	if !time.Now().Before(g.expiresAt) {
		return errWSAuthorization
	}
	return nil
}

// One data writer owns authorization, heartbeat and write deadlines. Closing the
// socket interrupts the reader; the caller then unregisters from its hub.
func serveAuthorizedWebSocket(conn *websocket.Conn, send <-chan []byte, guard *wsAuthorization, interval time.Duration) {
	defer conn.Close()
	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(3 * interval))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(3 * interval)) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var expiry <-chan time.Time
	if !guard.expiresAt.IsZero() {
		timer := time.NewTimer(time.Until(guard.expiresAt))
		defer timer.Stop()
		expiry = timer.C
	}
	deny := func() {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "authorization expired or revoked"), time.Now().Add(time.Second))
	}
	for {
		select {
		case <-done:
			return
		case <-guard.ctx.Done():
			return
		case <-expiry:
			deny()
			return
		case <-ticker.C:
			if guard.validate() != nil {
				deny()
				return
			}
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)) != nil {
				return
			}
		case msg, ok := <-send:
			if !ok {
				return
			}
			if guard.validate() != nil {
				deny()
				return
			}
			deadline := time.Now().Add(5 * time.Second)
			if !guard.expiresAt.IsZero() && guard.expiresAt.Before(deadline) {
				deadline = guard.expiresAt
			}
			if conn.SetWriteDeadline(deadline) != nil || conn.WriteMessage(websocket.TextMessage, msg) != nil {
				return
			}
		}
	}
}
