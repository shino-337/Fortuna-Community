package api

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

// insightAssigneeView is what the API says about a possible or current assignee.
type insightAssigneeView struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

// loadScopedInsight reads a finding and enforces the caller's cluster scope. An out-of-scope
// finding answers 404, so its existence does not leak.
func loadScopedInsight(db *gorm.DB, c *gin.Context) (models.Insight, bool) {
	var insight models.Insight
	if err := db.First(&insight, "id = ?", strings.TrimSpace(c.Param("id"))).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "finding not found"})
			return insight, false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read finding"})
		return insight, false
	}
	ok, err := insightInRequestScope(db, c, insight)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve finding scope"})
		return insight, false
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "finding not found"})
		return insight, false
	}
	return insight, true
}

// insightOwningCluster is the cluster a finding belongs to, from the finding or its pod.
func insightOwningCluster(db *gorm.DB, insight models.Insight) (string, error) {
	if id := strings.TrimSpace(insight.ClusterID); id != "" {
		return id, nil
	}
	return resourceUIDClusterID(db, insight.ResourceUID)
}

// userCanTriageFinding reports whether u could act on a finding in clusterID: an active account
// whose role grants findings.ack and whose cluster scope covers the cluster. A scoped user never
// qualifies for a finding whose cluster is unknown.
func userCanTriageFinding(u models.User, clusterID string) bool {
	if !u.Active {
		return false
	}
	granted := false
	for _, p := range authorization.PermissionsForUser(u.Role) {
		if p == authorization.PermissionFindingsAck {
			granted = true
			break
		}
	}
	if !granted {
		return false
	}
	if strings.EqualFold(authorization.NormalizeRole(u.Role), models.RoleAdmin) {
		return true
	}
	doc := authorization.ParseScopeDocument(u.ScopeJSON)
	if !doc.RestrictsClusters() {
		return true
	}
	return clusterID != "" && doc.ClusterAllowed(clusterID)
}

func requestUserID(c *gin.Context) (uint, bool) {
	if v, ok := c.Get("userID"); ok {
		if id, ok := v.(uint); ok && id > 0 {
			return id, true
		}
	}
	if raw, ok := c.Get("user"); ok {
		if u, ok := raw.(*models.User); ok && u != nil && u.ID > 0 {
			return u.ID, true
		}
	}
	return 0, false
}

// ListInsightAssignees lists the users a finding can be assigned to: everyone who could triage it.
// GET /risk/insights/:id/assignees
func ListInsightAssignees(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		insight, ok := loadScopedInsight(db, c)
		if !ok {
			return
		}
		clusterID, err := insightOwningCluster(db, insight)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve finding cluster"})
			return
		}
		var users []models.User
		if err := db.Where("active = ?", true).Order("username").Limit(500).Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read users"})
			return
		}
		items := make([]insightAssigneeView, 0, len(users))
		for _, u := range users {
			if userCanTriageFinding(u, clusterID) {
				items = append(items, insightAssigneeView{ID: u.ID, Username: u.Username})
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Username < items[j].Username })
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}

// AssignInsight sets or clears the finding's assignee. Body: {"userId": <id>} or {"userId": null}.
// The assignee must be able to triage the finding (role and cluster scope); closed findings
// keep the assignee they were closed with.
// PUT /risk/insights/:id/assignee
func AssignInsight(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			UserID *uint `json:"userId"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"userId\": <id or null>}"})
			return
		}
		insight, ok := loadScopedInsight(db, c)
		if !ok {
			return
		}
		if insight.Status == "resolved" || insight.Status == "dismissed" {
			c.JSON(http.StatusConflict, gin.H{"error": "closed findings cannot be reassigned"})
			return
		}

		before := map[string]any{"assigneeUserId": insight.AssigneeUserID, "assignee": insight.AssigneeUsername}
		updates := map[string]interface{}{"updated_at": time.Now()}
		var assignee *insightAssigneeView
		if body.UserID == nil {
			updates["assignee_user_id"] = nil
			updates["assignee_username"] = ""
			updates["assigned_at"] = nil
		} else {
			var target models.User
			if err := db.First(&target, "id = ?", *body.UserID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "this user cannot be assigned the finding"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read user"})
				return
			}
			clusterID, err := insightOwningCluster(db, insight)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not resolve finding cluster"})
				return
			}
			if !userCanTriageFinding(target, clusterID) {
				// Same answer as an unknown id, so the endpoint does not reveal which accounts exist.
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "this user cannot be assigned the finding"})
				return
			}
			now := time.Now()
			updates["assignee_user_id"] = target.ID
			updates["assignee_username"] = target.Username
			updates["assigned_at"] = now
			assignee = &insightAssigneeView{ID: target.ID, Username: target.Username}
		}

		if err := db.Model(&models.Insight{}).Where("id = ?", insight.ID).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save assignee"})
			return
		}
		if defaultRisksCache != nil {
			defaultRisksCache.ClearByPrefix("risks:list:")
		}

		after := map[string]any{"assigneeUserId": nil, "assignee": ""}
		if assignee != nil {
			after = map[string]any{"assigneeUserId": assignee.ID, "assignee": assignee.Username}
		}
		id := strconv.FormatUint(uint64(insight.ID), 10)
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(middleware.GrantedPermissions(c)),
			c.GetString(middleware.CtxJWTSessionID),
			securityaudit.ActionFindingsAssign,
			"finding",
			id,
			"success",
			securityaudit.ClassifyResultSeverity(securityaudit.ActionFindingsAssign, "success"),
			"jwt",
			before,
			after,
			nil,
			nil,
		)
		securityaudit.Append(db, &ev)

		c.JSON(http.StatusOK, gin.H{"assignee": assignee})
	}
}
