package models

import "time"

// PodNetworkConnection represents a network connection for a pod (Network Service).
type PodNetworkConnection struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	PodUID        string    `gorm:"type:varchar(255);not null;index" json:"podUid"`
	ClusterID     string    `gorm:"type:varchar(255);not null" json:"clusterId"`
	Namespace     string    `gorm:"type:varchar(255);not null" json:"namespace"`
	ContainerName string    `gorm:"type:varchar(255)" json:"containerName"`
	SourceIP      string    `gorm:"type:varchar(45)" json:"sourceIp"`
	SourcePort    int       `gorm:"default:0" json:"sourcePort"`
	DestIP        string    `gorm:"type:varchar(45)" json:"destIp"`
	DestPort      int       `gorm:"default:0" json:"destPort"`
	Protocol      string    `gorm:"type:varchar(16);default:tcp" json:"protocol"`
	State         string    `gorm:"type:varchar(32)" json:"state"`
	BytesSent     int64     `gorm:"default:0" json:"bytesSent"`
	BytesRecv     int64     `gorm:"default:0" json:"bytesRecv"`
	ObservedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"observedAt"`
	CreatedAt     time.Time `json:"createdAt"`
	RuntimeSource string    `gorm:"type:varchar(32);default:exec" json:"runtimeSource,omitempty"` // "host" | "exec" for UI indicator
}

// TableName overrides table name.
func (PodNetworkConnection) TableName() string {
	return "pod_network_connections"
}
