# Kubernetes Event Sync & Dashboard Architecture Proposal

## 1. Overview
This document summarizes the analysis of why the current system fails to record Kubernetes events, sync data to the core service, and reflect updated state in the dashboard. It also proposes a clean and scalable architecture for log ingestion, state synchronization, event processing, and UI updates.

---

## 2. Current Issues Identified

### 2.1 AuditLog Not Recorded
- Strict throttling prevents logs from being written.
- `shouldCreateAuditLog()` rejects most events due to a 5-minute duplicate suppression window.
- Full sync logic ignores most create/update events.
- DeltaSync detection incorrect → sync treated as FullSync → no events created.

### 2.2 Batch Buffer Never Flushed
- Logs collected in `auditLogBuffer` do not reach DB unless buffer hits 100 entries.
- If sync frequency is low → buffer never flushes.
- `flushAuditLogBuffer()` only triggered in `defer` → skipped on early returns or errors.

### 2.3 Delete Detection Broken
- Delete detection is mixed between full and delta syncs.
- DeltaSync does not properly handle delete events.
- Dashboard never sees deletion events.

### 2.4 Dashboard Uses AuditLog instead of State
- UI depends on AuditLog to reflect graph changes.
- When AuditLog is suppressed, UI sees “no changes” even though state has changed.

---

## 3. Root Causes

### 3.1 Conflation of State & Events
- AuditLog is currently used to build dashboard state, which is incorrect.
- State (current truth) and EventLog (history) must be separated.

### 3.2 Incorrect Sync Architecture
- Sync method mixes batch ingestion, delta detection, event creation, throttling, deletion logic, and DB write.
- No queue or buffer system for reliable ingestion.

### 3.3 Overly Aggressive Anti-Spam Logic
- Throttling based on time (5 minutes) is too broad.
- Should switch to diff-hash suppression.

---

## 4. Recommended Architecture

### 4.1 Component Separation

#### A. State Sync (Idempotent)
Handles:
- ServiceAccount
- Role / ClusterRole
- RoleBinding
- Namespace, Cluster

**Rules:**
- Always write the latest state.
- Overwrite existing entries.
- No throttling.
- No reliance on AuditLog.

#### B. Event / Audit Sync (Append-only)
Handles:
- Create / Delete / Modify events.
- Pod-uses-SA events.
- Token rotation.
- RBAC changes.

**Rules:**
- Append-only.
- TTL (7–90 days).
- No suppression except exact duplicate (diff-hash).

#### C. Watcher (Delta Events)
- Real-time ADDED / MODIFIED / DELETED.
- Directly updates State.
- Optionally generates AuditLog.

---

## 5. Pipeline Architecture

```
Agent → Core API → Queue → Event Processor → DB (State + Events)
                                          → Dashboard
```

### 5 Components
1. **Agent Collector**  
   - Full Sync snapshots  
   - Incremental updates

2. **Agent Watcher**  
   - Real-time Delta events

3. **Core API**
   - Accepts events
   - Validates and forwards to Queue
   - Writes minimal state

4. **Queue (Kafka/Redis/NATS)**
   - Backpressure
   - Guarantees no event loss

5. **Event Processor**
   - Merges events
   - Applies State changes
   - Writes AuditLog
   - Enforces TTL
   - Archives old data

---

## 6. Database Strategy

### 6.1 State DB (Truth)
- Postgres / Neo4j / DocumentDB
- Idempotent writes
- Used for dashboard graph rendering

### 6.2 Event DB (History)
- ClickHouse / Elastic / Loki / TimescaleDB
- TTL enforced
- Partitioned by day
- Used for audit timeline

### 6.3 Archival
- Events > 90 days → S3 / MinIO (Parquet)

---

## 7. Recommendations for Fixing Current System

### 7.1 Immediate Fixes
- Reduce throttling window to < 10 seconds.
- Flush AuditLog buffer at end of each sync block.
- Correct DeltaSync detection.
- Remove throttling on create/delete events.
- Ensure dashboard reads from State tables, not AuditLog.

### 7.2 Medium-Term Refactor
- Split StateSync and AuditSync.
- Move AuditLog logic to a background worker.
- Introduce a queue (Redis/Kafka).
- Use diff-hash to detect real changes.
- Clean up delete detection logic.

### 7.3 Long-Term Improvements
- Implement proper event summarization.
- Apply rate-limiting per agent, not per event type.
- Use partitioned indexes for performance.
- Replace time-based suppression with diff-hash suppression.

---

## 8. Proposed Event Processing Logic

### 8.1 Event Creation
```
if diff(state_before, state_after) != EMPTY:
    write state
    push event to queue
```

### 8.2 Duplicate Detection
```
if hash(eventPayload) == lastHash:
    skip
else:
    writeAudit(event)
```

### 8.3 Delete Logic
- FullSync → compare DB state vs agent snapshot → delete missing entries.
- DeltaSync → rely on watcher DELETE events.

---

## 9. Recommended Dashboard Data Flow

### UI Reads:
### ✔ STATE
- ServiceAccount
- Namespace
- RoleBinding
- Pod → SA mapping

### UI Uses Events Only For:
- Timeline
- Activity feed
- Security incident viewer

This ensures dashboard remains responsive and accurate even if AuditLog traffic is high.

---

## 10. Summary

### PROBLEMS:
- AuditLog suppression prevents updates from being visible.
- Batch buffer never flushes in small syncs.
- Incorrect delta/full sync logic.
- Dashboard using wrong data source.

### SOLUTION:
- Separate State and Event sync.
- Adopt queue-based ingestion.
- Rewrite throttling logic.
- Use State DB for graph rendering.
- Implement reliable deletion and delta detection.

---

## 11. Next Steps
- Refactor `agent_service.go` into:
  - `state_sync.go`
  - `event_sync.go`
  - `audit_worker.go`
  - `queue.go`
- Add ClickHouse or Elasticsearch for event storage.
- Introduce Redis/Kafka as ingestion buffer.
- Update dashboard backend to use State-first data model.