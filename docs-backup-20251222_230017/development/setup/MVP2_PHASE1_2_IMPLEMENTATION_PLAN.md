# MVP2 Phase 1.2: Risk Prioritization - Implementation Plan

**Date**: 2025-12-08  
**Status**: 🚀 **READY TO IMPLEMENT**

---

## 📊 Current Status Analysis

### ✅ Already Implemented (Phase 1.1)
1. **Priority Levels**: P0-P3 calculation in `models.GetPriorityLevel()`
2. **Priority Filtering**: Basic priority filter in `GetRiskScores` API
3. **Risk Scoring**: Complete algorithm with priority assignment
4. **Database Schema**: `priority_level` column with index

### ⏳ To Implement (Phase 1.2)

#### 1. Priority-Based Grouping APIs
- [ ] `GET /api/v1/risk/priorities` - Priority statistics
- [ ] `GET /api/v1/risk/top` - Top N risks
- [ ] `GET /api/v1/risk/grouped` - Grouped by various dimensions

#### 2. Grouping Strategies
- [ ] By Priority Level (P0, P1, P2, P3)
- [ ] By Cluster
- [ ] By Namespace
- [ ] By Resource Type
- [ ] By Attack Vector (from insight types)

#### 3. Enhanced Filtering
- [ ] Priority-based sorting
- [ ] Multi-priority filtering
- [ ] Priority range filtering

#### 4. Statistics & Analytics
- [ ] Priority distribution counts
- [ ] Average score by priority
- [ ] Risk trend by priority

---

## 🎯 Implementation Plan

### Step 1: Priority Statistics API
**Endpoint**: `GET /api/v1/risk/priorities`

**Response**:
```json
{
  "priorities": {
    "P0": {
      "count": 12,
      "avgScore": 95.5,
      "percentage": 20.7
    },
    "P1": {
      "count": 25,
      "avgScore": 78.3,
      "percentage": 43.1
    },
    "P2": {
      "count": 15,
      "avgScore": 55.2,
      "percentage": 25.9
    },
    "P3": {
      "count": 6,
      "avgScore": 25.8,
      "percentage": 10.3
    }
  },
  "total": 58,
  "highestPriority": "P0"
}
```

### Step 2: Top Risks API
**Endpoint**: `GET /api/v1/risk/top?limit=10&priority=P0`

**Response**:
```json
{
  "risks": [
    {
      "resource_uid": "abc-123",
      "resource_name": "nginx-pod",
      "namespace": "production",
      "cluster_id": "prod-1",
      "total_score": 98.5,
      "priority_level": "P0",
      "insights_count": 5,
      "highest_severity": "critical"
    }
  ],
  "limit": 10,
  "priority": "P0"
}
```

### Step 3: Grouped Risks API
**Endpoint**: `GET /api/v1/risk/grouped?by=cluster&priority=P0`

**Response**:
```json
{
  "grouped": {
    "prod-cluster-1": {
      "count": 8,
      "avgScore": 92.3,
      "risks": [...]
    },
    "dev-cluster-1": {
      "count": 4,
      "avgScore": 88.7,
      "risks": [...]
    }
  },
  "groupBy": "cluster",
  "priority": "P0"
}
```

### Step 4: Enhanced GetRiskScores
- Add `sortBy` parameter (score, priority, name, namespace)
- Add `groupBy` parameter (cluster, namespace, type, priority)
- Add multi-priority filter (`priority=P0,P1`)

---

## 📝 Implementation Details

### File Structure
```
KSAM/core/internal/api/risk/
├── risk_handlers.go (existing)
│   ├── GetRiskScores (enhance)
│   ├── GetRiskScore (existing)
│   ├── CalculateRiskScore (existing)
│   └── GetRiskTrends (existing)
│
└── priority_handlers.go (new)
    ├── GetPriorityStatistics
    ├── GetTopRisks
    └── GetGroupedRisks
```

### Database Queries

#### Priority Statistics
```sql
SELECT 
  priority_level,
  COUNT(*) as count,
  AVG(total_score) as avg_score,
  ROUND(COUNT(*) * 100.0 / (SELECT COUNT(*) FROM risk_scores WHERE deleted_at IS NULL), 2) as percentage
FROM risk_scores
WHERE deleted_at IS NULL
GROUP BY priority_level
ORDER BY 
  CASE priority_level
    WHEN 'P0' THEN 1
    WHEN 'P1' THEN 2
    WHEN 'P2' THEN 3
    WHEN 'P3' THEN 4
  END;
```

#### Top Risks
```sql
SELECT *
FROM risk_scores
WHERE deleted_at IS NULL
  AND priority_level = ?
ORDER BY total_score DESC
LIMIT ?;
```

#### Grouped by Cluster
```sql
SELECT 
  cluster_id,
  COUNT(*) as count,
  AVG(total_score) as avg_score,
  MAX(total_score) as max_score
FROM risk_scores
WHERE deleted_at IS NULL
  AND priority_level = ?
GROUP BY cluster_id
ORDER BY avg_score DESC;
```

---

## 🧪 Testing Plan

### Unit Tests
- [ ] Priority statistics calculation
- [ ] Top N selection
- [ ] Grouping logic
- [ ] Filtering combinations

### Integration Tests
- [ ] API endpoint responses
- [ ] Database query performance
- [ ] Pagination with grouping
- [ ] Error handling

### E2E Tests
- [ ] Create test resources with different priorities
- [ ] Verify priority statistics
- [ ] Verify top risks
- [ ] Verify grouped results

---

## 📊 Success Criteria

1. ✅ All priority-based APIs return correct data
2. ✅ Grouping works for all dimensions (cluster, namespace, type)
3. ✅ Top N risks correctly sorted by score
4. ✅ Priority statistics match database counts
5. ✅ API response time < 500ms for typical queries
6. ✅ All tests passing

---

## 🚀 Next Steps

1. Implement `GetPriorityStatistics` handler
2. Implement `GetTopRisks` handler
3. Implement `GetGroupedRisks` handler
4. Enhance `GetRiskScores` with new filters
5. Add routes in `routes.go`
6. Test all endpoints
7. Update documentation

---

**Estimated Effort**: 3 days (24 hours)  
**Priority**: HIGH (enables dashboard views)

