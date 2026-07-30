package securityaudit

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

var ErrImmutableAuditMutation = errors.New("security_activity_logs is append-only")

// ImmutableWriter is the only supported application interface for governance security activity rows.
type ImmutableWriter interface {
	Append(db *gorm.DB, e *Event)
}

type immutableWriter struct{}

// NewImmutableWriter returns the standard append-only audit writer.
func NewImmutableWriter() ImmutableWriter {
	return immutableWriter{}
}

func (immutableWriter) Append(db *gorm.DB, e *Event) {
	Append(db, e)
}

// RejectSecurityActivityMutation should be called if application code attempts UPDATE/DELETE on security_activity_logs.
func RejectSecurityActivityMutation(db *gorm.DB, c ContextMeta, attemptedOp string) {
	if db == nil || !db.Migrator().HasTable(&models.SecurityActivityLog{}) {
		return
	}
	ev := Event{
		EventID:      NewEventID(),
		Action:       ActionAuditTamperDenied,
		ResourceType: "audit_store",
		ResourceID:   attemptedOp,
		Result:       "deny",
		Severity:     "critical",
		AuthMethod:   "internal",
		ActorUsername: "system",
		Details: map[string]any{
			"attempted": attemptedOp,
			"context":   c,
		},
	}
	Append(db, &ev)
}

// ContextMeta is minimal non-request context for tamper-denied logging.
type ContextMeta map[string]any

// GuardSecurityActivityModel registers GORM callbacks that reject mutating statements on security_activity_logs.
// Best-effort: complements DB triggers; logs a governance event when a mutation is attempted.
func GuardSecurityActivityModel(db *gorm.DB) {
	if db == nil {
		return
	}
	_ = db.Callback().Update().Before("gorm:update").Register("fortuna:forbid_sal_update", func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || tx.Statement.Schema == nil {
			return
		}
		if tx.Statement.Schema.Table != (models.SecurityActivityLog{}).TableName() {
			return
		}
		RejectSecurityActivityMutation(tx, ContextMeta{"phase": "gorm_before_update"}, "UPDATE")
		tx.AddError(fmt.Errorf("%w", ErrImmutableAuditMutation))
	})
	_ = db.Callback().Delete().Before("gorm:delete").Register("fortuna:forbid_sal_delete", func(tx *gorm.DB) {
		if tx == nil || tx.Statement == nil || tx.Statement.Schema == nil {
			return
		}
		if tx.Statement.Schema.Table != (models.SecurityActivityLog{}).TableName() {
			return
		}
		RejectSecurityActivityMutation(tx, ContextMeta{"phase": "gorm_before_delete"}, "DELETE")
		tx.AddError(fmt.Errorf("%w", ErrImmutableAuditMutation))
	})
}
