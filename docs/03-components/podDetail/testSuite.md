# Integration Test Suite – Spec Hash & Async PCE Verification

## Environment Requirements

- 1 cluster (kind/minikube/real)
- Agent (new version)
- Core (migration 069 applied)
- Database accessible for verification
- Risk engine logging enabled
- Worker queue metrics enabled

---

# GROUP 1 — Spec Hash Correctness

## TC1.1 – Same Spec → Same Hash

Step:
1. Deploy pod A.
2. Capture spec_hash from DB.
3. Delete pod.
4. Recreate identical manifest.

Expected:
- spec_hash identical.
- Only 1 PCE run (create only).
- No additional PCE on stable state.

Verify:
SELECT spec_hash FROM pods WHERE name='pod-a';

---

## TC1.2 – Order Change Should NOT Change Hash

Modify:
- Reorder containers in manifest.
- Reorder env vars.

Expected:
- spec_hash unchanged.
- No PCE triggered.

If hash changes → canonicalization bug.

---

## TC1.3 – Status Change Should NOT Trigger PCE

Change:
- Induce CrashLoopBackOff (bad command).
- Phase changes Running → CrashLoop.

Expected:
- spec_hash unchanged.
- No PCE re-evaluation.

Verify:
Check PCE log count unchanged.

---

## TC1.4 – Security Context Change MUST Trigger PCE

Modify:
- Set privileged=true.

Expected:
- spec_hash changed.
- PCE triggered exactly once.
- Risk score updated.

Verify:
SELECT spec_hash, risk_score FROM pods;

---

# GROUP 2 — Async PCE Correctness

## TC2.1 – Sync Must Return Before PCE Completes

Inject artificial 5s delay in PCE worker.

Deploy pod.

Expected:
- Sync API returns immediately (<500ms).
- PCE log appears after delay.
- No sync timeout.

---

## TC2.2 – last_evaluated_hash Updated Correctly

Change spec twice quickly:

Step:
1. Deploy pod (hash A).
2. Modify spec (hash B).
3. Modify spec again (hash C).

Expected:
- Final DB:
  spec_hash = C
  last_evaluated_hash = C
- Risk reflects spec C only.

Verify:
SELECT spec_hash, last_evaluated_hash FROM pods;

---

# GROUP 3 — Race Condition Protection

## TC3.1 – Stale Worker Result Must Be Discarded

Procedure:

1. Deploy pod spec A.
2. Immediately update to spec B.
3. Artificially delay worker for spec A.
4. Let worker for spec A finish AFTER worker B.

Expected:
- DB risk reflects spec B.
- No overwrite by stale A.
- Log contains "discard stale evaluation".

---

## TC3.2 – High Frequency Updates

Loop 20 times:
- Patch annotation or toleration.

Expected:
- Only final hash evaluated.
- Worker queue size stable.
- No duplicate risk events.

---

# GROUP 4 — Backward Compatibility

## TC4.1 – Old Agent (No spec_hash)

Simulate:
- Disable specHash in agent.

Deploy pod.

Expected:
- PCE always triggered.
- spec_hash stored NULL.
- No crash.

---

## TC4.2 – Upgrade Agent

Steps:
1. Run old agent.
2. Deploy pod.
3. Upgrade agent.
4. Update spec.

Expected:
- New spec_hash stored.
- PCE triggered on first new hash.
- System stable.

---

# GROUP 5 — Soft Delete & Restore

## TC5.1 – Delete Pod

1. Deploy pod.
2. Delete pod.

Expected:
- deleted_at set.
- No PCE triggered.

---

## TC5.2 – Restore Pod With Same Spec

1. Deploy.
2. Delete.
3. Recreate identical.

Expected:
- spec_hash same.
- PCE triggered (restore path).
- Risk recalculated.

---

# GROUP 6 — Scale Test

## TC6.1 – 500 Pods Initial Sync

Create 500 pods.

Expected:
- Sync stable.
- Worker queue processes gradually.
- CPU spike controlled.
- No deadlock.

---

## TC6.2 – Agent Restart

1. 500 pods running.
2. Restart agent.

Expected:
- spec_hash unchanged.
- No PCE flood.
- Sync completes < defined SLA.

---

# GROUP 7 — Exposure to Status-Only Change

## TC7.1 – Restart Container

Kill container process.

Expected:
- restart_count updated.
- spec_hash unchanged.
- No PCE triggered.

---

# GROUP 8 — Database Consistency

## TC8.1 – Verify Index Usage

Run:
EXPLAIN SELECT * FROM pods WHERE cluster_id=? AND namespace=?;

Expected:
- Uses composite index.
- No full table scan.

---

# GROUP 9 — Failure Scenarios

## TC9.1 – Worker Crash Mid-Evaluation

Simulate panic in worker.

Expected:
- No corrupt risk.
- Job retried.
- spec_hash != last_evaluated_hash flagged.

---

## TC9.2 – Core Restart During Evaluation

1. Trigger PCE.
2. Restart Core before completion.

Expected:
- Job re-queued (if durable queue).
- Or safe state without inconsistent risk.

---

# GROUP 10 — Security Hardening

## TC10.1 – Tampered clusterId

Modify payload clusterId manually.

Expected:
- Rejected.
- mTLS identity enforced.

---

# Acceptance Criteria

System is considered production-ready if:

- No stale risk overwrite.
- No PCE flood on restart.
- Hash stable under order changes.
- Worker queue bounded.
- Risk always matches latest spec_hash.
- Sync latency unaffected by PCE.

---

# Test execution

- **Runner script:** `scripts/e2e/run-pod-detail-test-suite.sh`
- **Chạy:** `bash scripts/e2e/run-pod-detail-test-suite.sh`
- **Báo cáo:** `docs/test-results/pod-detail-test-suite-report-<timestamp>.md` (unit tests, DB verification, process info, mapping testSuite ↔ implementation).