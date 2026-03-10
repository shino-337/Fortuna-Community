package models

import "time"

// PodRuntimeMetrics stores per-container CPU/memory and state (Runtime Service).
type PodRuntimeMetrics struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	PodUID             string    `gorm:"type:varchar(255);not null;index" json:"podUid"`
	ClusterID          string    `gorm:"type:varchar(255);not null" json:"clusterId"`
	Namespace          string    `gorm:"type:varchar(255);not null" json:"namespace"`
	ContainerName      string    `gorm:"type:varchar(255);not null" json:"containerName"`
	CPUUsageMillicore  int       `gorm:"default:0" json:"cpuUsageMillicore"`
	MemoryUsageBytes   int64     `gorm:"default:0" json:"memoryUsageBytes"`
	MemoryLimitBytes   int64     `gorm:"default:0" json:"memoryLimitBytes"`
	RestartCount       int       `gorm:"default:0" json:"restartCount"`
	State              string    `gorm:"type:varchar(32);default:Running" json:"state"` // Running, Waiting, Terminated
	LastObservedAt     time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"lastObservedAt"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// TableName overrides table name.
func (PodRuntimeMetrics) TableName() string {
	return "pod_runtime_metrics"
}
