# Insights Management Guide

## Current Insights Summary

### Total Insights: 108

### Breakdown by Type and Severity

| Type | Severity | Count | Description |
|------|----------|-------|-------------|
| RBAC_CLUSTER_ADMIN | Critical | 3 | ServiceAccounts bound to cluster-admin role |
| RBAC_WILDCARD_PERMISSIONS | Critical | 2 | Roles with wildcard permissions |
| RBAC_WILDCARD_PERMISSIONS | High | 10 | Roles with wildcard permissions |
| RBAC_OVERPRIVILEGED_ROLE | High | 8 | Roles with excessive permissions |
| RBAC_OVERPRIVILEGED_BINDING | Medium | 5 | ServiceAccounts bound to overprivileged roles |
| RBAC_OVERPRIVILEGED_ROLE | Medium | 4 | Roles with excessive permissions |
| RBAC_ORPHAN_SERVICE_ACCOUNT | Low | 76 | ServiceAccounts not used by any pod |

## Ways to Add Insights

### 1. Automatic via Risk Worker (Stream Processing)
- **Method**: Risk Worker processes messages from NATS stream `ksam.normalized.>`
- **When**: Real-time as new resources are discovered
- **Engine**: `pkg/riskengine` (Risk Worker engine)
- **Status**: ✅ Operational, processes new messages only

### 2. Automatic via Scheduled Job
- **Method**: CronJob runs every 6 hours
- **When**: Periodic evaluation of all resources
- **Engine**: `pkg/riskengine` (Risk Worker engine via HistoricalRiskEvaluator)
- **Status**: ✅ Implemented
- **Configuration**: `deploy/risk-evaluation-cronjob.yaml`

### 3. Automatic via Internal Scheduler
- **Method**: Built-in scheduler in Core service
- **When**: Every 6 hours (configurable)
- **Engine**: `pkg/riskengine` (Risk Worker engine)
- **Status**: ✅ Implemented
- **Configuration**: `core/cmd/main.go`

### 4. Manual via API Endpoint (Internal Risk Engine)
- **Endpoint**: `POST /api/v1/insights/evaluate`
- **Method**: Uses `internal/risk` engine
- **When**: On-demand via API call
- **Engine**: `internal/risk` (different engine)
- **Status**: ✅ Operational

### 5. Manual via API Endpoint (Risk Worker Engine)
- **Endpoint**: `POST /api/v1/insights/evaluate/historical`
- **Method**: Uses `pkg/riskengine` engine (same as Risk Worker)
- **When**: On-demand via API call
- **Engine**: `pkg/riskengine` (Risk Worker engine)
- **Status**: ✅ Implemented

## Database Write Strategy

### Duplicate Prevention
The `InsightManager` uses improved duplicate detection:
1. **Unique Key**: Type + Severity + Resource Identifier (name + namespace)
2. **Matching Logic**: Checks if insight exists for same resource and risk type
3. **Update Strategy**: Updates existing insight if description/resources changed
4. **No Duplicates**: Prevents creating duplicate insights for same resource

### Conflict Avoidance
- **Transaction Safety**: Each insight creation/update uses database transaction
- **Idempotent Operations**: Same evaluation can run multiple times safely
- **Update vs Create**: Updates existing insights instead of creating duplicates
- **Timestamp Tracking**: `created_at` and `updated_at` track when insights were created/modified

## Scheduled Jobs

### Kubernetes CronJob
- **File**: `deploy/risk-evaluation-cronjob.yaml`
- **Schedule**: Every 6 hours (`0 */6 * * *`)
- **Method**: Calls API endpoint `/api/v1/insights/evaluate/historical`
- **Authentication**: Uses admin credentials

### Internal Scheduler
- **Location**: `core/internal/scheduler/risk_scheduler.go`
- **Schedule**: Every 6 hours (configurable)
- **Method**: Directly calls `HistoricalRiskEvaluator`
- **Startup**: Automatically starts with Core service

## API Endpoints

### 1. Trigger Risk Evaluation (Internal Engine)
```bash
curl -X POST http://localhost:8080/api/v1/insights/evaluate \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json"
```

### 2. Trigger Historical Risk Evaluation (Risk Worker Engine)
```bash
curl -X POST http://localhost:8080/api/v1/insights/evaluate/historical \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json"
```

### 3. Get All Insights
```bash
curl http://localhost:8080/api/v1/insights \
  -H "Authorization: Bearer <token>"
```

### 4. Get Insights Summary
```bash
curl http://localhost:8080/api/v1/insights/summary \
  -H "Authorization: Bearer <token>"
```

## Best Practices

1. **Use Risk Worker Engine for Consistency**: Use `/api/v1/insights/evaluate/historical` for manual triggers to ensure consistency with stream processing
2. **Scheduled Evaluation**: Let scheduled jobs handle periodic evaluation
3. **Manual Triggers**: Use API endpoints for on-demand evaluation after major changes
4. **Monitor Insights**: Regularly check insights summary to track security posture
5. **Resolve Critical Issues**: Prioritize Critical and High severity insights

## Troubleshooting

### Insights Not Created
1. Check Risk Worker logs: `kubectl logs -n ksam deployment/ksam-core | grep RiskWorker`
2. Check scheduled job: `kubectl logs -n ksam -l app=ksam-risk-evaluation --tail=50`
3. Verify database connectivity
4. Check for errors in Core logs

### Duplicate Insights
- The improved `InsightManager` should prevent duplicates
- If duplicates appear, check the matching logic in `insight_manager.go`

### Performance Issues
- Historical evaluation processes all resources - may take time for large clusters
- Consider adjusting scheduler interval if needed
- Monitor database size and performance

