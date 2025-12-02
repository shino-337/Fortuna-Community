# Risk Worker Debug Analysis

## Current Status
- **Risk Workers**: ✅ Subscribed to `ksam.normalized.>`
- **Insights Created**: ❌ 0 insights in database
- **Risk Worker Logs**: ⚠️ No logs found

## Issues Identified

### 1. Risk Worker Not Processing Messages
- Risk Workers are subscribed but not logging any activity
- Possible causes:
  - Messages not being delivered to Risk Workers
  - Risk Worker processing but failing silently
  - Risk Worker not matching any rules

### 2. Risk Engine Data Enrichment
- Updated Risk Engine to parse `raw_json` and merge with normalized data
- Added support for array field access (e.g., `subjects[].kind`)
- Enhanced logging in Risk Worker

### 3. CorrelatorWorker JSON Errors
- Still seeing `invalid input syntax for type json` errors for Pods
- This might be blocking some processing, but RoleBindings are being stored successfully

## Next Steps
1. Verify Risk Worker is receiving messages
2. Check if rules are matching resources
3. Test with a known risky resource (e.g., cluster-admin binding)
4. Add more detailed logging to Risk Engine evaluation

## Changes Made
- `engine.go`: Added `enrichResourceData()` to parse `raw_json`
- `engine.go`: Enhanced `getFieldValue()` to support array indexing
- `engine.go`: Updated `evaluateResourceCondition()` to handle array fields
- `risk_worker.go`: Enhanced logging for better visibility

