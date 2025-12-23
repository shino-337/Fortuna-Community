# MVP2 Phase 1.3: Risk Analytics - Implementation Plan

**Date**: 2025-12-08  
**Status**: 🚀 **READY TO IMPLEMENT**

---

## 📊 Overview

Phase 1.3 focuses on providing insights into risk trends and patterns through time-series analysis and correlation analysis.

---

## 🎯 Objectives

1. **Time-Series Analysis**: Daily, weekly, monthly risk score trends
2. **Correlation Analysis**: Risk vs deployments, cluster age, namespace activity
3. **Predictive Analytics** (Future): Forecast trends, pattern identification

---

## 📋 Implementation Plan

### 1. Time-Series Analysis APIs

#### `GET /api/v1/risk/analytics/trends`
- **Purpose**: Risk trends over different time periods
- **Parameters**:
  - `period` (daily, weekly, monthly, yearly)
  - `days` (number of days to analyze, default: 30)
  - `cluster` (optional)
  - `namespace` (optional)
- **Response**: Time-series data with risk score averages, counts by priority

#### `GET /api/v1/risk/analytics/comparison`
- **Purpose**: Compare risk metrics across time periods
- **Parameters**:
  - `compare` (week-over-week, month-over-month, year-over-year)
  - `cluster` (optional)
- **Response**: Comparison metrics (change percentage, trend direction)

### 2. Correlation Analysis APIs

#### `GET /api/v1/risk/analytics/correlation`
- **Purpose**: Analyze correlations between risk and other factors
- **Parameters**:
  - `factor` (deployments, cluster_age, namespace_activity, team_ownership)
  - `cluster` (optional)
- **Response**: Correlation coefficients and insights

### 3. Enhanced GetRiskTrends
- Already implemented in Phase 1.1
- May need enhancements for analytics use cases

---

## 📝 Database Queries

### Time-Series Aggregation
```sql
-- Daily averages
SELECT 
  DATE(calculated_at) as date,
  AVG(total_score) as avg_score,
  COUNT(*) as count,
  COUNT(CASE WHEN priority_level = 'P0' THEN 1 END) as p0_count,
  COUNT(CASE WHEN priority_level = 'P1' THEN 1 END) as p1_count
FROM risk_scores
WHERE calculated_at >= NOW() - INTERVAL '30 days'
  AND deleted_at IS NULL
GROUP BY DATE(calculated_at)
ORDER BY date;
```

### Week-over-Week Comparison
```sql
-- Current week vs previous week
WITH current_week AS (
  SELECT AVG(total_score) as avg_score
  FROM risk_scores
  WHERE calculated_at >= DATE_TRUNC('week', NOW())
    AND deleted_at IS NULL
),
previous_week AS (
  SELECT AVG(total_score) as avg_score
  FROM risk_scores
  WHERE calculated_at >= DATE_TRUNC('week', NOW()) - INTERVAL '1 week'
    AND calculated_at < DATE_TRUNC('week', NOW())
    AND deleted_at IS NULL
)
SELECT 
  current_week.avg_score as current,
  previous_week.avg_score as previous,
  (current_week.avg_score - previous_week.avg_score) as change
FROM current_week, previous_week;
```

---

## 🏗️ File Structure

```
KSAM/core/internal/api/risk/
├── risk_handlers.go (existing)
├── priority_handlers.go (existing)
└── analytics_handlers.go (new)
    ├── GetRiskTrendsAnalytics
    ├── GetRiskComparison
    └── GetRiskCorrelation
```

---

## 🧪 Testing Plan

1. **Unit Tests**: Time-series aggregation logic
2. **Integration Tests**: API endpoints with various parameters
3. **E2E Tests**: Verify analytics match database queries
4. **Performance Tests**: Ensure queries complete in <1s

---

## 📊 Success Criteria

- [ ] All analytics APIs return correct data
- [ ] Time-series data matches database aggregations
- [ ] Comparison metrics accurate
- [ ] Correlation analysis provides insights
- [ ] API response time < 500ms
- [ ] All tests passing

---

## 🚀 Estimated Effort

- **Time-Series Analysis**: 1-2 days (8-16 hours)
- **Correlation Analysis**: 1-2 days (8-16 hours)
- **Testing**: 0.5 day (4 hours)
- **Total**: 2.5-4.5 days (20-36 hours)

---

## 📋 Next Steps

1. Create `analytics_handlers.go`
2. Implement time-series aggregation
3. Implement comparison logic
4. Implement correlation analysis
5. Add routes
6. Test all endpoints
7. Update documentation

---

**Status**: Ready to start implementation

