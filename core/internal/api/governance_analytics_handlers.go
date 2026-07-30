package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/sessions"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

// GetGovernancePermissionExplorer returns permission taxonomy plus route coverage and role grants (system.audit.read).
func GetGovernancePermissionExplorer() gin.HandlerFunc {
	return func(c *gin.Context) {
		opts := RouteVerifyOptions{}
		inv := FortunaRouteSecurityInventory(opts)
		routesByPerm := make(map[string][]RouteSecuritySpec)
		for _, r := range inv {
			if r.PrimaryPermission == "" {
				continue
			}
			k := string(r.PrimaryPermission)
			routesByPerm[k] = append(routesByPerm[k], r)
		}

		type row struct {
			Permission        string   `json:"permission"`
			Level             string   `json:"level"`
			Destructive       bool     `json:"destructive"`
			RiskBand          string   `json:"riskBand"`
			Roles             []string `json:"roles"`
			RouteCount        int      `json:"routeCount"`
			Routes            []string `json:"routes"`
			GraphTierExposure bool     `json:"graphTierExposure"`
		}
		out := make([]row, 0)
		for _, p := range authorization.AllPermissions() {
			lvl := authorization.ClassifyPermission(p)
			destructive := lvl == authorization.LevelDestructive
			risk := "LOW"
			switch lvl {
			case authorization.LevelDestructive, authorization.LevelSecurityCritical:
				risk = "CRITICAL"
			case authorization.LevelPlatform:
				risk = "HIGH"
			case authorization.LevelWrite:
				risk = "MEDIUM"
			}
			rs := routesByPerm[string(p)]
			rn := make([]string, 0, len(rs))
			graphExp := false
			for _, x := range rs {
				rn = append(rn, x.Method+" "+x.Path)
				if x.GraphClass != "" && x.GraphClass != graphNone {
					graphExp = true
				}
			}
			out = append(out, row{
				Permission:        string(p),
				Level:             string(lvl),
				Destructive:       destructive,
				RiskBand:          risk,
				Roles:             authorization.RolesGrantingPermission(p),
				RouteCount:        len(rs),
				Routes:            rn,
				GraphTierExposure: graphExp,
			})
		}
		c.JSON(http.StatusOK, gin.H{"items": out, "generatedAt": time.Now().UTC().Format(time.RFC3339)})
	}
}

// GetGovernanceAccessReview returns privilege hygiene signals (system.audit.read).
func GetGovernanceAccessReview(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		const dormantDays = 90
		const staleDays = 180
		now := time.Now().UTC()
		var users []models.User
		if err := db.Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		type signal struct {
			Code       string         `json:"code"`
			Severity   string         `json:"severity"`
			UserID     uint           `json:"userId,omitempty"`
			Username   string         `json:"username,omitempty"`
			Role       string         `json:"role,omitempty"`
			Detail     map[string]any `json:"detail,omitempty"`
		}
		var signals []signal

		adminCount := 0
		for _, u := range users {
			role := strings.ToLower(strings.TrimSpace(u.Role))
			if role == models.RoleAdmin {
				adminCount++
			}
			last := u.LastLogin
			dormant := !last.IsZero() && now.Sub(last) > dormantDays*24*time.Hour
			if last.IsZero() {
				dormant = now.Sub(u.CreatedAt) > dormantDays*24*time.Hour
			}
			if role == models.RoleAdmin && dormant && u.Active {
				signals = append(signals, signal{
					Code:     "DORMANT_ADMIN",
					Severity: "CRITICAL",
					UserID:   u.ID,
					Username: u.Username,
					Role:     u.Role,
					Detail: map[string]any{
						"lastLogin": last,
					},
				})
			}
			stale := !last.IsZero() && now.Sub(last) > staleDays*24*time.Hour
			if last.IsZero() {
				stale = now.Sub(u.CreatedAt) > staleDays*24*time.Hour
			}
			if stale && u.Active && role != models.RoleViewer {
				signals = append(signals, signal{
					Code:     "STALE_ACCOUNT",
					Severity: "HIGH",
					UserID:   u.ID,
					Username: u.Username,
					Role:     u.Role,
				})
			}
			doc := authorization.ParseScopeDocument(u.ScopeJSON)
			if doc.RestrictsClusters() && doc.ClusterAllowListSize() > 8 {
				signals = append(signals, signal{
					Code:     "EXCESSIVE_SCOPE",
					Severity: "MEDIUM",
					UserID:   u.ID,
					Username: u.Username,
					Detail: map[string]any{"clusterCount": doc.ClusterAllowListSize()},
				})
			}
		}
		if adminCount > 5 {
			signals = append(signals, signal{
				Code:     "PRIVILEGE_CONCENTRATION",
				Severity: "HIGH",
				Detail:   map[string]any{"adminCount": adminCount},
			})
		}

		// Inactive sessions (idle > 30d, not revoked)
		if sessions.TableExists(db) {
			cutoff := now.Add(-30 * 24 * time.Hour)
			var idle int64
			_ = db.Model(&models.UserSession{}).
				Where("revoked_at IS NULL AND expires_at > ? AND last_activity_at < ?", now, cutoff).
				Count(&idle).Error
			if idle > 0 {
				signals = append(signals, signal{
					Code:     "INACTIVE_SESSIONS",
					Severity: "MEDIUM",
					Detail:   map[string]any{"approxCount": idle},
				})
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"signals":     signals,
			"userTotal":   len(users),
			"generatedAt": now.Format(time.RFC3339),
		})
	}
}

// GetGovernanceCorrelationSignals returns lightweight anomaly primitives over security_activity_logs (system.audit.read).
func GetGovernanceCorrelationSignals(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !db.Migrator().HasTable(&models.SecurityActivityLog{}) {
			c.JSON(http.StatusOK, gin.H{"signals": []any{}, "message": "security_activity_logs not present"})
			return
		}
		since := time.Now().UTC().Add(-24 * time.Hour)
		type agg struct {
			ActorUserID uint   `gorm:"column:actor_user_id"`
			Action      string `gorm:"column:action"`
			C           int64  `gorm:"column:c"`
		}
		var rows []agg
		// SQLite + Postgres compatible minimal aggregation
		if err := db.Raw(`
SELECT actor_user_id, action, COUNT(*) AS c
FROM security_activity_logs
WHERE created_at >= ?
GROUP BY actor_user_id, action
HAVING COUNT(*) >= 8
ORDER BY c DESC
LIMIT 50`, since).Scan(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		type sig struct {
			Code     string         `json:"code"`
			Severity string         `json:"severity"`
			Detail   map[string]any `json:"detail"`
		}
		var out []sig
		for _, r := range rows {
			code := "EVENT_BURST"
			sev := "MEDIUM"
			if strings.Contains(strings.ToLower(r.Action), "deny") || strings.Contains(strings.ToLower(r.Action), "authz") {
				code = "AUTHZ_DENY_BURST"
				sev = "HIGH"
			}
			if strings.Contains(strings.ToLower(r.Action), "graph") {
				code = "GRAPH_ACTIVITY_BURST"
			}
			if strings.Contains(strings.ToLower(r.Action), "export") {
				code = "EXPORT_BURST"
				sev = "HIGH"
			}
			out = append(out, sig{
				Code:     code,
				Severity: sev,
				Detail: map[string]any{
					"actorUserId": r.ActorUserID,
					"action":      r.Action,
					"count24h":    r.C,
				},
			})
		}

		// Revocation spikes (aggregate)
		var rev int64
		_ = db.Model(&models.SecurityActivityLog{}).
			Where("created_at >= ? AND (action = ? OR action = ?)", since, "session_revoke", "sessions_revoke_all").
			Count(&rev).Error
		if rev >= 10 {
			out = append(out, sig{Code: "SESSION_REVOCATION_SPIKE", Severity: "MEDIUM", Detail: map[string]any{"count24h": rev}})
		}

		c.JSON(http.StatusOK, gin.H{"signals": out, "windowHours": 24, "generatedAt": time.Now().UTC().Format(time.RFC3339)})
	}
}

// ListInvestigationEvents is an alias of the security activity feed with investigation-oriented defaults (system.audit.read).
func ListInvestigationEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("investigation_limit_default", "100")
		ListSecurityActivity(db)(c)
	}
}
