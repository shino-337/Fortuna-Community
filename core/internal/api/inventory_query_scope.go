package api

import "gorm.io/gorm"

// column is supplied by handlers, never by request input.
func scopedInventoryQuery(q *gorm.DB, scope riskGovernanceScope, column string) *gorm.DB {
	if scope.clusterID != "" {
		q = q.Where(column+" = ?", scope.clusterID)
	}
	if scope.restricted {
		q = q.Where(column+" IN ?", scope.clusterIDs)
	}
	return q
}
