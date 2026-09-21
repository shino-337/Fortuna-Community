package models

import (
	"time"

	"github.com/lib/pq"
)

// PodCapability stores evaluated offensive capabilities for a pod.
type PodCapability struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ClusterID       string         `gorm:"type:varchar(255);index;uniqueIndex:idx_pod_capability_identity" json:"clusterId,omitempty"`
	PodUID          string         `gorm:"type:varchar(255);not null;index;uniqueIndex:idx_pod_capability_identity" json:"podUid"`
	Namespace       string         `gorm:"type:varchar(255);not null;index" json:"namespace"`
	CapabilityID    string         `gorm:"type:varchar(100);not null;index;uniqueIndex:idx_pod_capability_identity" json:"capabilityId"`
	CapabilityGroup string         `gorm:"type:varchar(50);not null;index;column:capability_group" json:"group"`
	Severity        string         `gorm:"type:varchar(20);not null;index" json:"severity"`
	State           string         `gorm:"type:varchar(20);default:detected;index;check:state IN ('detected', 'confirmed', 'exploited', 'chained')" json:"state"`
	Confidence      float64        `gorm:"type:float;default:0.5;check:confidence >= 0 AND confidence <= 1" json:"confidence"`
	CapabilityClass string         `gorm:"type:varchar(30);default:effective;index;column:capability_class" json:"capabilityClass,omitempty"`
	DerivedFrom     string         `gorm:"type:jsonb;column:derived_from" json:"derivedFrom,omitempty"`
	FirstSeenAt     *time.Time     `gorm:"type:timestamp with time zone" json:"firstSeenAt,omitempty"`
	LastSeenAt      *time.Time     `gorm:"type:timestamp with time zone" json:"lastSeenAt,omitempty"`
	Evidence        string         `gorm:"type:jsonb" json:"evidence"`
	MitreTechniques pq.StringArray `gorm:"type:text[];column:mitre" json:"mitreTechniques"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

// TableName overrides table name.
func (PodCapability) TableName() string {
	return "pod_capabilities"
}
