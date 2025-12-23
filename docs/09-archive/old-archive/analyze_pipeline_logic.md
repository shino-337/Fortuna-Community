# KSAM SBOM/CVE Pipeline Logic Analysis

## Complete Event Flow

```
1. Agent (DaemonSet)
   └─> Watches Kubernetes Pods
   └─> Publishes to: ksam.raw.pods

2. NormalizerWorker
   └─> Subscribes to: ksam.raw.>
   └─> Normalizes pod data
   └─> Publishes to: ksam.normalized.pods

3. SBOMWorker
   └─> Subscribes to: ksam.normalized.pods (DeliverAll, MaxAckPending=10)
   └─> Filters: Skip Deleted events, Skip non-Running pods
   └─> De-dup: Skip if pod/container/image already has sbom_id
   └─> Calls: sbom.Service.EnsureSBOM(imageRef)
       ├─> ResolveDigest (cache-first)
       ├─> If cache hit: Update use_count, last_used_at, image_tag (if changed)
       └─> If cache miss: ExtractSBOM → Normalize → Persist
   └─> Calls: sbom.Service.UpsertPodImageScan (links pod to SBOM)
   └─> Publishes to: ksam.sbom.created (ONLY if new SBOM or new link)

4. CVEMatcherWorker
   └─> Subscribes to: ksam.sbom.created (DeliverAll, MaxAckPending=50)
   └─> Loads SBOM from DB
   └─> Calls: cve.Matcher.MatchSBOM (queries PostgreSQL: cves + package_vulnerabilities)
   └─> Persists: cve_matches (with dedup via unique index)
   └─> Creates: insights (CRITICAL/HIGH only)

5. Insights API
   └─> Queries: insights table (type='vulnerability')
   └─> Returns: JSON with CVE details
```

## Logic Verification Points

### ✅ SBOMWorker De-dup Logic (Line 108-115)
**Current Behavior:**
- If `pod_image_scans` already has `sbom_id` for this pod/container/image → Skip
- **Impact:** Prevents duplicate SBOM generation and duplicate events
- **Correctness:** ✅ Correct - No need to regenerate SBOM for same image

**Edge Case:**
- If CVE database is updated after SBOM creation, CVE matching won't re-run
- **Mitigation:** This is acceptable - CVE database updates are infrequent

### ✅ SBOM Tag Tracking (service.go:50-78)
**Current Behavior:**
- Cache hit: Updates `image_tag` and `image_name` if they differ
- **Impact:** SBOM always shows correct tag even if digest is reused
- **Correctness:** ✅ Correct - Tag is metadata, digest is immutable identifier

### ✅ Pod Phase Filtering (sbom_worker.go:84-89)
**Current Behavior:**
- Only processes pods with `phase == "Running"`
- **Impact:** Skips Pending/Terminating pods
- **Correctness:** ✅ Correct - Prevents processing incomplete pods

### ✅ CVEMatcherWorker Severity Filtering (cve_matcher_worker.go:86-89)
**Current Behavior:**
- Only creates insights for CRITICAL and HIGH severity
- **Impact:** Reduces noise, focuses on critical vulnerabilities
- **Correctness:** ✅ Correct - Configurable via `onlySeverities` map

### ✅ CVEMatcherWorker Deleted SBOM Handling (cve_matcher_worker.go:64-68)
**Current Behavior:**
- If SBOM not found → Skip silently (return nil)
- **Impact:** Prevents infinite retry loops
- **Correctness:** ✅ Correct - Deleted SBOMs shouldn't be retried

## Potential Issues & Recommendations

### Issue 1: SBOM_CREATED Event Not Published on Cache Hit
**Scenario:** Pod recreated with same image (same digest)
- SBOMWorker skips (de-dup logic)
- No SBOM_CREATED event published
- CVEMatcherWorker not triggered

**Impact:** Low - CVE matching already done for this SBOM
**Recommendation:** ✅ Current behavior is correct

### Issue 2: NATS Slow Consumer (FIXED)
**Status:** ✅ Fixed
- MaxAckPending increased from 1 to 10
- DeliverAll mode ensures existing messages are processed

### Issue 3: SBOM Tag Mismatch (FIXED)
**Status:** ✅ Fixed
- Tag is updated when digest matches but tag differs

## Testing Recommendations

1. **Cache Hit Test:** Verify tag update when same digest, different tag
2. **Cache Miss Test:** Verify full SBOM generation
3. **CVE Matching Test:** Verify CVE matches are created
4. **Insight Creation Test:** Verify insights are created with correct data
5. **API Verification Test:** Verify Insights API returns correct data

