# E2E Verification Checklist

Complete checklist for verifying end-to-end functionality from Pod creation to Insight generation.

## 📋 Pre-Test Setup

- [ ] Core service is running and healthy
- [ ] Agent service is running and connected to Core
- [ ] Database is accessible and migrations are applied
- [ ] NATS is running and accessible
- [ ] Kubernetes cluster is accessible
- [ ] Test namespace is created (default: `ksam-test`)
- [ ] Test images are available (nginx, alpine, etc.)

## 🔄 Phase 1: Pod Creation & Registration

### Pod Creation
- [ ] Pod is created successfully in Kubernetes
- [ ] Pod reaches `Running` state
- [ ] Pod has valid container images
- [ ] Pod metadata is correct (name, namespace, UID, labels, annotations)

### Agent Registration
- [ ] Agent detects new pod via watch mechanism
- [ ] Agent registers with Core (gRPC `RegisterAgent`)
- [ ] Agent receives registration confirmation
- [ ] Agent connection is established (mTLS if enabled)

**Verification Points:**
- [ ] Check Core logs for agent registration
- [ ] Verify agent appears in database (if tracked)
- [ ] Confirm gRPC connection is active

**Timing Measurement:**
- [ ] Time from pod creation to agent detection: `___ seconds`
- [ ] Time from agent detection to registration: `___ seconds`

## 📦 Phase 2: SBOM Extraction

### SBOM Extraction Trigger
- [ ] Agent triggers SBOM extraction for pod containers
- [ ] SBOM extraction starts within expected time window
- [ ] Multiple containers are processed (if applicable)

### SBOM Data Quality
- [ ] SBOM contains valid package information
- [ ] Package names are correct
- [ ] Package versions are correct
- [ ] PURLs are generated correctly
- [ ] Ecosystem is identified correctly (npm, pypi, maven, etc.)
- [ ] Component count matches expected packages

### SBOM Storage
- [ ] SBOM is stored in database (`sboms` table)
- [ ] SBOM ID is generated
- [ ] `pod_uid` matches actual pod UID
- [ ] `container_name` matches actual container name
- [ ] `image_digest` is captured (if available)
- [ ] `extracted_at` timestamp is set
- [ ] `agent_id` is recorded (if applicable)

### SBOM Components
- [ ] SBOM components are stored (`sbom_components` table)
- [ ] Component count matches extracted packages
- [ ] Component names match package names
- [ ] Component versions match package versions
- [ ] PURLs are stored correctly
- [ ] Component metadata is preserved

**Verification Points:**
- [ ] Query database: `SELECT * FROM sboms WHERE pod_uid = '<pod_uid>'`
- [ ] Query database: `SELECT COUNT(*) FROM sbom_components WHERE sbom_id = '<sbom_id>'`
- [ ] Check Core logs for SBOM receipt
- [ ] Verify NATS message published to `ksam-raw` stream

**Timing Measurement:**
- [ ] Time from pod creation to SBOM extraction start: `___ seconds`
- [ ] Time for SBOM extraction: `___ seconds`
- [ ] Time from SBOM extraction to database storage: `___ seconds`
- [ ] Total SBOM processing time: `___ seconds`

## 🔍 Phase 3: CVE Matching

### CVE Matching Trigger
- [ ] CVE matching is triggered after SBOM storage
- [ ] NATS message is published to `ksam-normalized` stream
- [ ] CVEMatcherWorker picks up the message
- [ ] CVE matching starts within expected time window

### CVE Database Lookup
- [ ] CVE database is accessible
- [ ] Package vulnerabilities are queried correctly
- [ ] Version range matching works correctly
- [ ] Multiple ecosystems are handled (npm, pypi, maven, etc.)
- [ ] Bulk queries are used (no N+1 queries)

### CVE Match Results
- [ ] CVE matches are found (if vulnerabilities exist)
- [ ] CVE IDs are correct
- [ ] Severity levels are assigned correctly (CRITICAL, HIGH, MEDIUM, LOW)
- [ ] CVSS scores are captured
- [ ] Affected version ranges are recorded
- [ ] Fixed version information is captured (if available)

### CVE Match Storage
- [ ] CVE matches are stored in database (`cve_matches` table)
- [ ] `sbom_id` references correct SBOM
- [ ] `package_name` matches component name
- [ ] `package_version` matches component version
- [ ] `cve_id` is correct
- [ ] `severity` is set correctly
- [ ] `cvss_score` is captured
- [ ] `matched_by` contains version range
- [ ] `matched_at` timestamp is set
- [ ] No duplicate matches (unique constraint works)

**Verification Points:**
- [ ] Query database: `SELECT * FROM cve_matches WHERE sbom_id = '<sbom_id>'`
- [ ] Verify CVE IDs exist in `package_vulnerabilities` table
- [ ] Check Core logs for CVE matching activity
- [ ] Verify NATS message published to `ksam-events` stream

**Timing Measurement:**
- [ ] Time from SBOM storage to CVE matching start: `___ seconds`
- [ ] Time for CVE database lookup: `___ seconds`
- [ ] Time for CVE matching logic: `___ seconds`
- [ ] Time from CVE matching to database storage: `___ seconds`
- [ ] Total CVE matching time: `___ seconds`

## 💡 Phase 4: Insight Generation

### Insight Generation Trigger
- [ ] Insight generation is triggered after CVE matching
- [ ] RiskWorker picks up CVE match events
- [ ] Risk engine evaluates vulnerabilities
- [ ] Insight generation starts within expected time window

### Risk Evaluation
- [ ] Risk scores are calculated correctly
- [ ] Severity levels are considered
- [ ] CVSS scores are factored in
- [ ] Exploit availability is checked
- [ ] Resource context is evaluated
- [ ] Risk thresholds are applied

### Insight Creation
- [ ] Insights are created for each vulnerability
- [ ] `insight_type` is set correctly (VULNERABILITY, etc.)
- [ ] `resource_type` matches pod type
- [ ] `resource_name` matches pod name
- [ ] `resource_namespace` matches pod namespace
- [ ] `resource_uid` matches pod UID
- [ ] `cve_id` references correct CVE
- [ ] `severity` matches CVE severity
- [ ] `cvss` score is captured
- [ ] `description` contains vulnerability details
- [ ] `recommendation` contains remediation advice
- [ ] `affected_component` matches package name
- [ ] `affected_version` matches package version
- [ ] `status` is set to `ACTIVE`
- [ ] `detected_at` timestamp is set

### Insight Deduplication
- [ ] Duplicate insights are prevented
- [ ] Unique constraint works: `(resource_uid, cve_id, insight_type)`
- [ ] Existing insights are updated (not duplicated)
- [ ] Status transitions are handled correctly

### Insight Storage
- [ ] Insights are stored in database (`insights` table)
- [ ] Batch UPSERT is used (efficient)
- [ ] All required fields are populated
- [ ] Soft delete works (`deleted_at` is NULL for active insights)

**Verification Points:**
- [ ] Query database: `SELECT * FROM insights WHERE resource_uid = '<pod_uid>'`
- [ ] Verify insight count matches CVE match count
- [ ] Check Core logs for insight creation
- [ ] Verify no duplicate insights exist

**Timing Measurement:**
- [ ] Time from CVE matching to insight generation start: `___ seconds`
- [ ] Time for risk evaluation: `___ seconds`
- [ ] Time for insight creation: `___ seconds`
- [ ] Time from insight creation to database storage: `___ seconds`
- [ ] Total insight generation time: `___ seconds`

## 🌐 Phase 5: API Verification

### API Endpoints
- [ ] `/health` endpoint returns 200 OK
- [ ] `/ready` endpoint returns 200 OK
- [ ] `/api/v1/insights` endpoint is accessible
- [ ] Authentication works (if enabled)

### Insights API Response
- [ ] GET `/api/v1/insights` returns list of insights
- [ ] Response includes newly created insights
- [ ] Response format matches API specification
- [ ] Pagination works (if implemented)
- [ ] Filtering works (by resource, severity, type, etc.)

### API Response Data Quality
- [ ] `id` matches database ID
- [ ] `insight_type` matches database value
- [ ] `resource_type` matches database value
- [ ] `resource_name` matches database value
- [ ] `resource_namespace` matches database value
- [ ] `resource_uid` matches database value
- [ ] `cve_id` matches database value
- [ ] `severity` matches database value
- [ ] `cvss` matches database value
- [ ] `description` matches database value
- [ ] `recommendation` matches database value
- [ ] `status` matches database value
- [ ] `detected_at` matches database timestamp
- [ ] `created_at` matches database timestamp
- [ ] `updated_at` matches database timestamp

### API Response vs Database
- [ ] All insights in database appear in API response
- [ ] Insight count matches: API count = Database count
- [ ] Field values match between API and database
- [ ] Timestamps are formatted correctly
- [ ] No missing or extra fields

**Verification Points:**
- [ ] Query API: `curl http://localhost:8080/api/v1/insights`
- [ ] Query database: `SELECT COUNT(*) FROM insights WHERE resource_uid = '<pod_uid>'`
- [ ] Compare API response JSON with database records
- [ ] Verify field mappings are correct

**Timing Measurement:**
- [ ] Time from insight storage to API availability: `___ seconds`
- [ ] API response time: `___ seconds`
- [ ] Total end-to-end time (pod creation → API response): `___ seconds`

## 🔄 Phase 6: Reconciliation

### SBOM Reconciliation
- [ ] Reconciler runs on schedule (every hour)
- [ ] Missing SBOMs are detected (pods without SBOM)
- [ ] Orphaned SBOMs are detected (SBOMs for deleted pods)
- [ ] Reconciliation actions are triggered
- [ ] Reconciliation logs are generated

**Verification Points:**
- [ ] Check Core logs for reconciliation activity
- [ ] Verify reconciliation runs on schedule
- [ ] Confirm missing/orphaned SBOMs are handled

**Timing Measurement:**
- [ ] Reconciliation cycle time: `___ seconds`
- [ ] Time to detect missing SBOMs: `___ seconds`
- [ ] Time to detect orphaned SBOMs: `___ seconds`

## 📊 Phase 7: Performance Verification

### Query Performance
- [ ] SBOM queries are fast (< 100ms)
- [ ] CVE match queries are fast (< 200ms)
- [ ] Insight queries are fast (< 100ms)
- [ ] Database indexes are used (EXPLAIN shows index usage)

### Processing Performance
- [ ] SBOM processing: < 30 seconds for typical image
- [ ] CVE matching: < 5 seconds for 200 packages
- [ ] Insight generation: < 2 seconds for 100 CVEs
- [ ] Total E2E time: < 60 seconds

### Database Performance
- [ ] No N+1 queries detected
- [ ] Batch operations are used
- [ ] Connection pool is utilized correctly
- [ ] No connection pool exhaustion

**Verification Points:**
- [ ] Check database query logs
- [ ] Monitor connection pool metrics
- [ ] Review NATS message processing times
- [ ] Analyze Core logs for performance issues

## ✅ Final Verification

### Data Consistency
- [ ] All data is consistent across services
- [ ] No orphaned records in database
- [ ] Foreign key relationships are valid
- [ ] Timestamps are logical (created_at < updated_at)

### Error Handling
- [ ] Errors are logged correctly
- [ ] Failed operations are retried
- [ ] Dead letter queue works (if applicable)
- [ ] System recovers from errors gracefully

### Monitoring
- [ ] Prometheus metrics are exported
- [ ] Metrics are accurate
- [ ] Alerts are configured (if applicable)
- [ ] Logs are structured and searchable

## 📝 Test Results Summary

**Test Execution Date:** `___`
**Test Executor:** `___`
**Environment:** `___`

**Results:**
- Total Test Cases: `___`
- Passed: `___`
- Failed: `___`
- Skipped: `___`

**Performance Summary:**
- Pod Creation → SBOM Extraction: `___ seconds`
- SBOM Extraction → CVE Matching: `___ seconds`
- CVE Matching → Insight Generation: `___ seconds`
- Insight Generation → API Response: `___ seconds`
- **Total E2E Time:** `___ seconds`

**Issues Found:**
- [ ] List any issues or discrepancies found during testing

**Recommendations:**
- [ ] List any recommendations for improvement

