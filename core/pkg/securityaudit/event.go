package securityaudit

import (
	"time"

	"github.com/google/uuid"
)

// Event is a normalized governance audit record (mapped to security_activity_logs).
type Event struct {
	EventID       string
	Timestamp     time.Time
	ActorID       uint
	ActorUsername string
	ActorRole     string
	ActorPerms    []string

	SourceIP  string
	UserAgent string
	SessionID string
	RequestID string
	CorrelID  string

	Action       string
	ResourceType string
	ResourceID   string
	TargetUserID *uint

	BeforeState any
	AfterState  any
	Details     any

	Severity   string // low|medium|high|critical
	AuthMethod string // jwt|password|dev_principal|unknown
	Result     string // success|deny|error
}

// NewEventID returns a RFC4122 UUID string for event_id.
func NewEventID() string {
	return uuid.NewString()
}
