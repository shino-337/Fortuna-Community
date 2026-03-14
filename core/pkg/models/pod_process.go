package models

import "time"

// PodProcess represents a process in a pod container (Process Service snapshot).
// History is kept: each POST adds a new snapshot (same observed_at per batch).
// Retention job deletes rows with observed_at older than POD_PROCESS_RETENTION_DAYS (default 30).
// API stores truncated strings: command 1024, binary_path 512, user_name 128, container_name 256.
type PodProcess struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	PodUID        string     `gorm:"type:varchar(255);not null;index" json:"podUid"`
	ClusterID     string     `gorm:"type:varchar(255);not null" json:"clusterId"`
	Namespace     string     `gorm:"type:varchar(255);not null" json:"namespace"`
	ContainerName string     `gorm:"type:varchar(255);not null" json:"containerName"`
	PID           int        `gorm:"column:p_id;not null" json:"pid"`
	PPID          int        `gorm:"column:pp_id;default:0" json:"ppid"`
	UserName      string     `gorm:"column:user_name;type:varchar(255)" json:"userName"`
	CPUPercent    float64    `gorm:"type:float;default:0" json:"cpuPercent"`
	MemoryPercent float64    `gorm:"type:float;default:0" json:"memoryPercent"`
	Command       string     `gorm:"type:text" json:"command"`
	BinaryPath    string     `gorm:"type:varchar(1024)" json:"binaryPath"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	ObservedAt    time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"observedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
	RuntimeSource string     `gorm:"type:varchar(32);default:exec" json:"runtimeSource,omitempty"` // "host" | "exec" for UI indicator
}

// TableName overrides table name.
func (PodProcess) TableName() string {
	return "pod_processes"
}
