package investigation

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Timeline event types (append-only).
const (
	EventCaseCreated       = "case.created"
	EventCaseArchived      = "case.archived"
	EventStatusChanged     = "case.status_changed"
	EventAssignmentChanged = "case.assignment_changed"
	EventCollaborationUpdated = "case.collaboration_updated"
	EventEntityPinned      = "entity.pinned"
	EventEntityUnpinned    = "entity.unpinned"
	EventRemediationAdded  = "remediation.added"
	EventRemediationUpdated = "remediation.updated"
	EventRemediationRemoved = "remediation.removed"
	EventNoteAdded         = "note.added"
	EventHandoffNote       = "handoff.note"
	EventPivot             = "case.pivot"
)

// AppendTimeline persists an immutable investigation timeline row (best-effort).
func AppendTimeline(db *gorm.DB, c *gin.Context, caseID, eventType, summary string, before, after, details any) {
	if db == nil || !db.Migrator().HasTable(&models.InvestigationActivityLog{}) {
		return
	}
	actorID := uint(0)
	actorName := "system"
	if u, ok := c.Get("user"); ok && u != nil {
		if actor, ok2 := u.(*models.User); ok2 && actor != nil {
			actorID = actor.ID
			actorName = actor.Username
		}
	}
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	detailsJSON, _ := json.Marshal(details)
	correl := c.GetHeader("X-Correlation-ID")
	if correl == "" {
		correl = c.GetHeader("X-Request-ID")
	}
	row := models.InvestigationActivityLog{
		CaseID:        caseID,
		EventID:       uuid.NewString(),
		ActorUserID:   actorID,
		ActorUsername: actorName,
		EventType:     eventType,
		Summary:       summary,
		BeforeJSON:    string(beforeJSON),
		AfterJSON:     string(afterJSON),
		DetailsJSON:   string(detailsJSON),
		CorrelationID: correl,
		RequestID:     c.GetHeader("X-Request-ID"),
		CreatedAt:     time.Now().UTC(),
	}
	_ = db.Create(&row).Error
}
