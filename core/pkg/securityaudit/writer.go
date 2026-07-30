package securityaudit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Append persists a single governance event (INSERT only). Fails silently on missing table.
func Append(db *gorm.DB, e *Event) {
	if db == nil || e == nil || !db.Migrator().HasTable(&models.SecurityActivityLog{}) {
		return
	}
	if e.EventID == "" {
		e.EventID = NewEventID()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	permsJSON, _ := json.Marshal(e.ActorPerms)
	beforeJSON, _ := json.Marshal(e.BeforeState)
	afterJSON, _ := json.Marshal(e.AfterState)
	detailsJSON, _ := json.Marshal(e.Details)

	row := models.SecurityActivityLog{
		EventID:         e.EventID,
		ActorUserID:     e.ActorID,
		ActorUsername:   e.ActorUsername,
		ActorRole:       e.ActorRole,
		PermissionsJSON: string(permsJSON),
		Action:          e.Action,
		Resource:        e.ResourceType,
		ResourceType:    e.ResourceType,
		ResourceID:      e.ResourceID,
		Result:          e.Result,
		Severity:        e.Severity,
		TargetUserID:    e.TargetUserID,
		RequestID:       e.RequestID,
		SessionID:       e.SessionID,
		CorrelationID:   e.CorrelID,
		SourceIP:        e.SourceIP,
		UserAgent:       e.UserAgent,
		AuthMethod:      e.AuthMethod,
		BeforeStateJSON: string(beforeJSON),
		AfterStateJSON:  string(afterJSON),
		DetailsJSON:     string(detailsJSON),
		CreatedAt:       e.Timestamp,
	}
	if err := db.Create(&row).Error; err != nil {
		return
	}
	forwardSIEM(e)
}

func forwardSIEM(e *Event) {
	url := strings.TrimSpace(os.Getenv("FORTUNA_SIEM_AUDIT_WEBHOOK"))
	if url == "" {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	go func(payload []byte, hook string) {
		client := &http.Client{Timeout: 2 * time.Second}
		req, err := http.NewRequest(http.MethodPost, hook, bytes.NewReader(payload))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		_, _ = client.Do(req)
	}(b, url)
}
