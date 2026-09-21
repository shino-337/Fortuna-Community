package api

import "time"

// podNetworkTopDestRow is one aggregated row: remote endpoint seen from this pod (dest tuple), ordered by observation count.
type podNetworkTopDestRow struct {
	DestIP              string    `json:"destIp" gorm:"column:dest_ip"`
	DestPort            int       `json:"destPort" gorm:"column:dest_port"`
	Protocol            string    `json:"protocol" gorm:"column:protocol"`
	ObservationCount    int64     `json:"observationCount" gorm:"column:observation_count"`
	LastObservedAt      time.Time `json:"lastObservedAt" gorm:"column:last_observed_at"`
	DistinctBucketCount int64     `json:"distinctBucketCount" gorm:"column:distinct_bucket_count"`
}
