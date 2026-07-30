package securityaudit

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/fortuna/core/pkg/models"
)

// CorrelationID prefers X-Correlation-ID then X-Request-ID.
func CorrelationID(c *gin.Context) string {
	if s := strings.TrimSpace(c.GetHeader("X-Correlation-ID")); s != "" {
		return s
	}
	return strings.TrimSpace(c.GetHeader("X-Request-ID"))
}

// FromRequest builds a normalized Event from the HTTP context (callers supply resolved permissions + session id).
func FromRequest(c *gin.Context, actorPerms []string, sessionID, action, resourceType, resourceID, result, severity, authMethod string, before, after, details any, targetUserID *uint) Event {
	e := Event{
		EventID:       NewEventID(),
		ActorPerms:    actorPerms,
		Action:        action,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		Result:        result,
		Severity:      severity,
		AuthMethod:    authMethod,
		SessionID:     sessionID,
		RequestID:     strings.TrimSpace(c.GetHeader("X-Request-ID")),
		CorrelID:      CorrelationID(c),
		SourceIP:      c.ClientIP(),
		UserAgent:     strings.TrimSpace(c.Request.UserAgent()),
		BeforeState:   before,
		AfterState:    after,
		Details:       details,
		TargetUserID:  targetUserID,
	}
	if u, ok := c.Get("user"); ok && u != nil {
		if actor, ok2 := u.(*models.User); ok2 && actor != nil {
			e.ActorID = actor.ID
			e.ActorUsername = actor.Username
			e.ActorRole = actor.Role
		}
	}
	if e.AuthMethod == "" {
		if src, ok := c.Get("auth_source"); ok {
			if s, ok2 := src.(string); ok2 {
				e.AuthMethod = s
			}
		}
	}
	// Normalize JSON-friendly before/after when plain maps
	e.BeforeState = normalizeJSONable(before)
	e.AfterState = normalizeJSONable(after)
	e.Details = normalizeJSONable(details)
	return e
}

func normalizeJSONable(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string, bool, float64, int, int32, int64, uint, uint32, uint64:
		return t
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return map[string]string{"_error": "marshal_failed"}
		}
		var out any
		if err := json.Unmarshal(b, &out); err != nil {
			return map[string]string{"_error": "unmarshal_failed"}
		}
		return out
	}
}
