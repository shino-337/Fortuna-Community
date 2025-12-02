# Risk Worker Analysis and Next Steps

## Current Status

### ✅ Completed
1. **Risk Engine Implementation**: 
   - 5 built-in risk rules defined
   - Risk evaluation engine with condition matching
   - Insight manager for creating/updating insights
   - Enhanced data enrichment to parse `raw_json`

2. **Risk Worker Integration**:
   - Risk Worker added to worker pool
   - Subscribed to `ksam.normalized.>` stream
   - Enhanced logging added

3. **Data Available**:
   - ClusterRoleBinding `minikube-rbac` exists with `cluster-admin` role
   - Should trigger `cis-5.1.3` rule (Critical severity)

### ❌ Issues
1. **No Insights Created**: Database shows 0 insights
2. **No Risk Worker Logs**: Risk Worker not logging any activity
3. **Possible Causes**:
   - Risk Worker only processes NEW messages from NATS (not historical data)
   - Subscription issue preventing message delivery
   - Risk Worker processing but rules not matching

## Root Cause Analysis

### Hypothesis 1: Risk Worker Only Processes New Messages
- **Evidence**: Data exists in database but was processed before Risk Worker was added
- **Solution**: Trigger Agent re-sync or manually trigger risk evaluation

### Hypothesis 2: Rules Not Matching Data Format
- **Evidence**: `roleRef` and `subjects` are stored as JSON strings in database
- **Solution**: Risk Engine now parses `raw_json` to extract structured data

### Hypothesis 3: Subscription Issue
- **Evidence**: Risk Workers show "consumer is already bound" errors on restart
- **Solution**: Use unique durable consumer names or clear NATS consumers

## Next Steps

### Immediate Actions
1. **Trigger Risk Evaluation Manually**:
   - Use API endpoint `/api/v1/insights/evaluate` to trigger evaluation
   - Or restart Agent to re-send inventory items

2. **Verify Risk Worker Subscription**:
   - Check NATS consumer status
   - Verify messages are being delivered to Risk Workers

3. **Test with Known Risky Resource**:
   - Create a test ClusterRoleBinding with cluster-admin
   - Verify Risk Worker processes it and creates insight

### Long-term Improvements
1. **Historical Risk Evaluation**:
   - Add endpoint to evaluate all existing resources
   - Schedule periodic re-evaluation

2. **Better Error Handling**:
   - Add retry logic for failed evaluations
   - Improve logging for debugging

3. **Rule Testing**:
   - Create test cases for each rule
   - Verify rules match expected patterns

## Test Plan

1. **Manual Trigger Test**:
   ```bash
   curl -X POST http://localhost:8080/api/v1/insights/evaluate
   ```

2. **New Resource Test**:
   - Create test ClusterRoleBinding
   - Wait for Agent to collect
   - Verify Risk Worker processes and creates insight

3. **Database Query Test**:
   - Query insights table
   - Verify insights are created with correct metadata

## Files Modified
- `core/pkg/riskengine/engine.go`: Added data enrichment, array field support
- `core/pkg/worker/risk_worker.go`: Enhanced logging
- `core/pkg/riskengine/rule.go`: 5 built-in rules defined
- `core/pkg/riskengine/insight_manager.go`: Insight creation/update logic

