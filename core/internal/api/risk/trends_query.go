package risk

import (
	"fmt"
	"time"

	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

// Aggregate in the database so response allocation depends on period buckets,
// rather than the number and width of all matching risk score observations.
func loadRiskTrendPoints(db *gorm.DB, scope analyticsScope, period, namespace string, cutoff time.Time) ([]TrendPoint, error) {
	var bucket string
	switch db.Dialector.Name() {
	case "postgres":
		format, unit := "YYYY-MM-DD", "day"
		switch period {
		case "weekly":
			unit = "week"
		case "monthly":
			unit = "month"
			format = "YYYY-MM"
		case "yearly":
			unit = "year"
			format = "YYYY"
		}
		bucket = fmt.Sprintf("to_char(date_trunc('%s',calculated_at AT TIME ZONE 'UTC'),'%s')", unit, format)
	case "sqlite":
		switch period {
		case "weekly":
			bucket = "date(calculated_at, '-' || ((CAST(strftime('%w',calculated_at) AS INTEGER)+6)%7) || ' days')"
		case "monthly":
			bucket = "strftime('%Y-%m',calculated_at)"
		case "yearly":
			bucket = "strftime('%Y',calculated_at)"
		default:
			bucket = "strftime('%Y-%m-%d',calculated_at)"
		}
	default:
		return nil, fmt.Errorf("unsupported trend aggregation database")
	}
	query := scope.apply(db.Model(&models.RiskScore{}), "cluster_id").Where("calculated_at >= ?", cutoff)
	if namespace != "" {
		query = query.Where("namespace = ?", namespace)
	}
	selectSQL := bucket + ` AS date, AVG(COALESCE(total_score,0)) AS avg_score, COUNT(*) AS count,
 SUM(CASE WHEN priority_level='P0' THEN 1 ELSE 0 END) AS p0_count,
 SUM(CASE WHEN priority_level='P1' THEN 1 ELSE 0 END) AS p1_count,
 SUM(CASE WHEN priority_level='P2' THEN 1 ELSE 0 END) AS p2_count,
 SUM(CASE WHEN priority_level='P3' THEN 1 ELSE 0 END) AS p3_count,
 CASE WHEN MAX(COALESCE(total_score,0))<0 THEN 0 ELSE MAX(COALESCE(total_score,0)) END AS max_score,
 CASE WHEN MIN(COALESCE(total_score,0))>100 THEN 0 ELSE MIN(COALESCE(total_score,0)) END AS min_score`
	trends := []TrendPoint{}
	err := query.Select(selectSQL).Group(bucket).Order(bucket + " ASC").Scan(&trends).Error
	return trends, err
}
