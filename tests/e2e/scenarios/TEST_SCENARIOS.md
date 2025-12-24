# E2E Test Scenarios

Complete test scenarios for end-to-end verification of KSAM platform.

## Scenario 1: Basic Pod-to-Insight Flow

**Objective:** Verify complete flow from pod creation to insight generation

**Steps:**
1. Create test pod with nginx image
2. Wait for pod to be ready
3. Verify SBOM extraction
4. Verify CVE matching
5. Verify insight generation
6. Verify API response
7. Compare API with database

**Expected Results:**
- Pod created successfully
- SBOM extracted within 30 seconds
- CVE matches found (if vulnerabilities exist)
- Insights generated for all CVEs
- API returns matching insights
- All data consistent between API and database

**Script:** `pod-to-insight-flow.sh`

---

## Scenario 2: Multi-Container Pod

**Objective:** Verify SBOM extraction for pods with multiple containers

**Steps:**
1. Create pod with multiple containers
2. Verify SBOM extraction for each container
3. Verify CVE matching for all containers
4. Verify insights are generated per container

**Expected Results:**
- SBOM extracted for each container
- CVE matches found per container
- Insights generated per container
- Container names are correctly associated

**Script:** `multi-container-pod-test.sh` (to be created)

---

## Scenario 3: High Vulnerability Image

**Objective:** Verify system handles images with many vulnerabilities

**Steps:**
1. Create pod with known vulnerable image
2. Verify SBOM extraction
3. Verify CVE matching (should find many CVEs)
4. Verify insight generation (should create many insights)
5. Verify performance is acceptable

**Expected Results:**
- System handles high CVE count gracefully
- Processing time is acceptable (< 60 seconds total)
- All insights are generated correctly
- No performance degradation

**Script:** `high-vulnerability-test.sh` (to be created)

---

## Scenario 4: No Vulnerability Image

**Objective:** Verify system handles images with no vulnerabilities

**Steps:**
1. Create pod with clean image (no known vulnerabilities)
2. Verify SBOM extraction
3. Verify CVE matching (should find no CVEs)
4. Verify no insights are generated
5. Verify API returns empty list

**Expected Results:**
- SBOM extracted successfully
- CVE matching completes (no matches found)
- No insights generated
- API returns empty list
- No errors or warnings

**Script:** `no-vulnerability-test.sh` (to be created)

---

## Scenario 5: Pod Deletion and Cleanup

**Objective:** Verify reconciliation handles deleted pods

**Steps:**
1. Create test pod
2. Wait for SBOM extraction
3. Delete pod
4. Wait for reconciliation cycle
5. Verify orphaned SBOM handling

**Expected Results:**
- Pod deleted successfully
- Reconciliation detects orphaned SBOM
- Cleanup actions are taken (if configured)
- No errors in logs

**Script:** `pod-deletion-test.sh` (to be created)

---

## Scenario 6: Rapid Pod Creation

**Objective:** Verify system handles rapid pod creation

**Steps:**
1. Create 10 pods rapidly
2. Verify all SBOMs are extracted
3. Verify all CVE matches are processed
4. Verify all insights are generated
5. Verify performance is acceptable

**Expected Results:**
- All pods processed successfully
- No message loss
- Processing time scales linearly
- No errors or timeouts

**Script:** `rapid-pod-creation-test.sh` (to be created)

---

## Scenario 7: API Consistency

**Objective:** Verify API response matches database exactly

**Steps:**
1. Create test pod
2. Wait for complete processing
3. Query database for insights
4. Query API for insights
5. Compare field-by-field

**Expected Results:**
- API count matches database count
- All fields match between API and database
- Timestamps are formatted correctly
- No missing or extra fields

**Script:** `compare-api-db.sh`

---

## Scenario 8: Performance Benchmark

**Objective:** Measure and verify performance targets

**Steps:**
1. Run SBOM processing benchmark
2. Run CVE matching benchmark
3. Run insight generation benchmark
4. Compare with performance targets

**Expected Results:**
- SBOM processing: < 30 seconds
- CVE matching: < 5 seconds for 200 packages
- Insight generation: < 2 seconds for 100 CVEs
- Total E2E: < 60 seconds

**Scripts:**
- `measure-sbom-processing.sh`
- `measure-cve-matching.sh`
- `measure-insight-generation.sh`

---

## Test Execution Order

Recommended execution order for comprehensive testing:

1. **Scenario 1** - Basic flow (smoke test)
2. **Scenario 4** - No vulnerabilities (edge case)
3. **Scenario 3** - High vulnerabilities (stress test)
4. **Scenario 2** - Multi-container (feature test)
5. **Scenario 7** - API consistency (data integrity)
6. **Scenario 8** - Performance (benchmark)
7. **Scenario 5** - Pod deletion (cleanup)
8. **Scenario 6** - Rapid creation (load test)

## Test Data Requirements

- Test namespace: `ksam-test`
- Test images:
  - `nginx:latest` - Standard image
  - `alpine:latest` - Minimal image
  - `node:18` - High vulnerability image
  - `python:3.9` - Medium vulnerability image

## Success Criteria

All scenarios should:
- Complete without errors
- Meet performance targets
- Maintain data consistency
- Generate appropriate logs
- Produce verifiable results

