# Graph Query API Implementation

**Date**: 2025-12-02  
**Status**: ✅ COMPLETED  
**Issue**: #6.2 from Architecture Review

---

## Summary

Implemented advanced graph query API for attack path analysis, permission queries, and risky pod detection using Apache AGE.

---

## Implementation Details

### Files Created

1. **`KSAM/core/pkg/graph/types.go`**
   - Defines types: `AttackPath`, `PathNode`, `PathEdge`, `Permission`, `PolicyRule`, `RiskyPod`

2. **`KSAM/core/pkg/graph/query_service.go`**
   - `QueryService` struct with methods:
     - `GetAttackPath()` - Finds attack paths from pod to admin roles
     - `GetServiceAccountPermissions()` - Gets permissions via graph traversal
     - `GetPodsWithEscalationRisk()` - Finds pods with privilege escalation risk

### Files Modified

1. **`KSAM/core/pkg/graph/age_engine.go`**
   - Added `GetSQLDB()` method for QueryService access
   - Added convenience methods: `GetAttackPath()`, `GetServiceAccountPermissionsGraph()`, `GetPodsWithEscalationRisk()`

2. **`KSAM/core/internal/api/graph_handlers.go`**
   - Added `GetAttackPaths()` handler
   - Added `GetServiceAccountPermissionsGraph()` handler
   - Added `GetRiskyPods()` handler

3. **`KSAM/core/internal/api/routes.go`**
   - Registered new routes:
     - `GET /api/v1/graph/attack-paths/:uid`
     - `GET /api/v1/graph/permissions/:uid`
     - `GET /api/v1/graph/risky-pods`

---

## API Endpoints

### 1. Get Attack Paths

**Endpoint**: `GET /api/v1/graph/attack-paths/:uid?max_depth=5`

**Description**: Finds attack paths from a pod to sensitive resources (admin roles).

**Parameters**:
- `uid` (path): Pod UID
- `max_depth` (query, optional): Maximum path depth (1-10, default: 5)

**Response**:
```json
{
  "pod_uid": "pod-123",
  "paths": [
    {
      "nodes": [...],
      "edges": [...],
      "total_risk": 7.5,
      "difficulty": 0.3,
      "impact": 0.9,
      "length": 2,
      "description": "Attack path from Pod to admin role"
    }
  ],
  "count": 1
}
```

### 2. Get Service Account Permissions (Graph-based)

**Endpoint**: `GET /api/v1/graph/permissions/:uid`

**Description**: Gets all permissions for a ServiceAccount via graph traversal.

**Parameters**:
- `uid` (path): ServiceAccount UID

**Response**:
```json
{
  "service_account_uid": "sa-123",
  "permissions": [
    {
      "role_name": "admin",
      "rules": [
        {
          "verbs": ["*"],
          "resources": ["*"],
          "api_groups": [""]
        }
      ],
      "namespace": "default",
      "role_type": "Role"
    }
  ],
  "count": 1
}
```

### 3. Get Risky Pods

**Endpoint**: `GET /api/v1/graph/risky-pods`

**Description**: Finds pods with privilege escalation risk.

**Response**:
```json
{
  "risky_pods": [
    {
      "uid": "pod-123",
      "name": "nginx",
      "namespace": "default",
      "service_account_name": "default",
      "role_name": "cluster-admin",
      "risk_score": 10.0,
      "risk_reason": "Pod uses ServiceAccount bound to cluster-admin role"
    }
  ],
  "count": 1
}
```

---

## Fallback Behavior

All graph queries gracefully handle the case when Apache AGE is not available:

- If AGE is not enabled, queries return empty results (not errors)
- Logs indicate when AGE is not available
- System continues to function normally

---

## Testing

To test the new endpoints:

```bash
# 1. Get attack paths for a pod
curl http://localhost:8080/api/v1/graph/attack-paths/pod-uid-123?max_depth=5

# 2. Get permissions for a ServiceAccount
curl http://localhost:8080/api/v1/graph/permissions/sa-uid-123

# 3. Get risky pods
curl http://localhost:8080/api/v1/graph/risky-pods
```

---

## Notes

1. **Cypher Query Syntax**: Queries use Apache AGE Cypher syntax with parameter binding via `jsonb_build_object()`.

2. **Path Parsing**: Current implementation has simplified path parsing. In production, would need proper AGTYPE parsing for complex path structures.

3. **Performance**: Queries are limited to 50 results to prevent performance issues.

4. **Error Handling**: All queries handle errors gracefully and return empty results if AGE is not available.

---

## Next Steps

1. ✅ Graph Query API implemented
2. ⏳ Test with AGE enabled
3. ⏳ Enhance path parsing for complex structures
4. ⏳ Add caching for frequently accessed queries
5. ⏳ Add metrics for query performance

---

**Status**: ✅ Implementation Complete  
**Ready for**: Testing and integration

