package models

import "time"

// RuntimeEventIngestClaim is the idempotency boundary for authenticated runtime
// v2 ingest. Historical runtime_events rows are intentionally not rewritten:
// only events accepted through the new contract participate in this claim table.
type RuntimeEventIngestClaim struct {
	ClusterID     string    `gorm:"primaryKey;size:255" json:"clusterId"`
	EventID       string    `gorm:"primaryKey;size:64" json:"eventId"`
	PayloadSHA256 string    `gorm:"size:64;not null" json:"payloadSha256"`
	AcceptedAt    time.Time `gorm:"index" json:"acceptedAt"`
}
