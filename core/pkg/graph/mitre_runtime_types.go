package graph

import "time"

// runtimeEventLite is a projection of runtime_events for MITRE enrichment and boost.
type runtimeEventLite struct {
	Mitre         string    `gorm:"column:mitre_technique"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	NodeName      string    `gorm:"column:node_name"`
	PodUID        string    `gorm:"column:pod_uid"`
	PodNamespace  string    `gorm:"column:pod_namespace"`
}
