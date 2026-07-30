package models

import "time"

// RuntimeEvent stores raw runtime probe events (sensor/audit).
type RuntimeEvent struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	EventID    string     `gorm:"type:varchar(64);index" json:"eventId,omitempty"` // canonical id for replay/idempotency (agent provided)
	ObservedAt *time.Time `gorm:"index" json:"observedAt,omitempty"`
	IngestedAt *time.Time `gorm:"index" json:"ingestedAt,omitempty"`

	ResolutionState string `gorm:"type:varchar(20);index" json:"resolutionState,omitempty"` // resolved|partial|unresolved
	SourceKind      string `gorm:"type:varchar(64);index" json:"sourceKind,omitempty"`
	SourceSensorID  string `gorm:"type:varchar(128);index" json:"sourceSensorId,omitempty"`
	SourceRule      string `gorm:"type:varchar(256)" json:"sourceRule,omitempty"`

	PayloadJSON string `gorm:"type:jsonb" json:"payloadJson,omitempty"`
	PayloadHash string `gorm:"type:varchar(128);index" json:"payloadHash,omitempty"`

	PodName    string    `gorm:"type:varchar(255)" json:"podName,omitempty"`
	PodUID     string    `gorm:"type:varchar(255);not null;index" json:"podUid"`
	Namespace  string    `gorm:"type:varchar(255);not null;index" json:"namespace"`
	NodeName   string    `gorm:"type:varchar(255)" json:"nodeName,omitempty"`
	Runtime    string    `gorm:"type:varchar(64)" json:"runtime,omitempty"`                               // e.g. "ebpf", "falco", "agent"
	EventType  string    `gorm:"type:varchar(128)" json:"eventType,omitempty"`                            // e.g. "runtime.exec"
	Signal     string    `gorm:"type:varchar(128)" json:"signal,omitempty"`                               // e.g. EBPF_EXEC_EVENT
	Mitre      string    `gorm:"column:mitre_technique;type:varchar(64)" json:"mitreTechnique,omitempty"` // e.g. T1059
	Severity   string    `gorm:"type:varchar(32)" json:"severity,omitempty"`                              // e.g. low/medium/high/critical
	Confidence float64   `gorm:"column:confidence" json:"confidence"`                                     // required on ingest; > 0 for scoring use
	Syscall    string    `gorm:"type:varchar(100);not null;index" json:"syscall"`
	TargetPath string    `gorm:"type:varchar(500)" json:"targetPath"`
	Capability string    `gorm:"type:varchar(100)" json:"capability"`
	CreatedAt  time.Time `json:"createdAt"`
}

// TableName overrides table name.
func (RuntimeEvent) TableName() string {
	return "runtime_events"
}
