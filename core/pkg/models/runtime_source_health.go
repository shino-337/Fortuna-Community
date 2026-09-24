package models

import "time"

// RuntimeSourceHealth is the latest independently observed upstream source-health
// lease for one authenticated runtime producer execution session.
type RuntimeSourceHealth struct {
	ClusterID   string    `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID     string    `gorm:"primaryKey;size:255" json:"agentId"`
	ProducerID  string    `gorm:"primaryKey;size:128" json:"producerId"`
	SessionID   string    `gorm:"size:128;index" json:"sessionId"`
	SourceKind  string    `gorm:"size:64;index" json:"sourceKind"`
	Status      string    `gorm:"size:32;index" json:"status"`
	ProofKind   string    `gorm:"size:64" json:"proofKind"`
	ObservedAt  time.Time `gorm:"index" json:"observedAt"`
	ValidUntil  time.Time `gorm:"index" json:"validUntil"`
	ReceivedAt  time.Time `gorm:"index" json:"receivedAt"`
	Reason      string    `gorm:"size:256" json:"reason,omitempty"`
}

func (h RuntimeSourceHealth) AuthoritativeFor(sessionID, sourceKind string, at time.Time) bool {
	return h.SessionID == sessionID &&
		h.SourceKind == sourceKind &&
		h.Status == "healthy" &&
		!h.ObservedAt.IsZero() &&
		!h.ValidUntil.IsZero() &&
		!at.Before(h.ObservedAt) &&
		at.Before(h.ValidUntil)
}
