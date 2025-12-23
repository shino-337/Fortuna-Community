# Testing Scripts

**85 scripts** for testing Fortuna components, pipelines, and integrations.

---

## 📂 Structure

```
testing/
├── e2e/                    ← 19 End-to-end tests
├── integration/            ← 8 Integration tests
├── performance/            ← 4 Performance tests
├── verification/           ← 7 Verification scripts
└── *.sh                    ← 47 Component/feature tests
```

---

## 🎯 Quick Start

### Run All Tests
```bash
./run_all_tests.sh
```

### Run E2E Tests
```bash
cd e2e
./test_e2e_comprehensive.sh
```

### Verify System
```bash
cd verification
./verify_insights_complete.sh
```

---

## 📁 Categories

### 🧪 E2E Tests (`e2e/` - 19 scripts)

Full end-to-end pipeline tests:
- Pod → SBOM → CVE → Insights flow
- Complete system tests
- MVP1/MVP2 comprehensive tests

**Key Scripts:**
- `test_e2e_comprehensive.sh` - Comprehensive E2E test
- `test-e2e-cve-pipeline.sh` - CVE pipeline test
- `test-sbom-end-to-end.sh` - SBOM pipeline test
- `full_e2e_test.sh` - Full system test

**Usage:**
```bash
cd e2e
./test_e2e_comprehensive.sh
```

---

### 🔗 Integration Tests (`integration/` - 8 scripts)

Component integration tests:
- API integration
- Data flow between components
- Migration tests
- Runtime environment tests

**Key Scripts:**
- `test_integration.sh` - General integration test
- `test_all_apis.sh` - Test all API endpoints
- `test_data_flow.sh` - Data flow verification
- `test_migration.sh` - Database migration test

**Usage:**
```bash
cd integration
./test_all_apis.sh
```

---

### ⚡ Performance Tests (`performance/` - 4 scripts)

Performance and load testing:
- Worker backpressure
- SBOM generation performance
- System benchmarks

**Key Scripts:**
- `test_performance.sh` - General performance test
- `test_worker_backpressure.sh` - Worker stress test
- `test_backpressure.sh` - System backpressure test

**Usage:**
```bash
cd performance
./test_performance.sh
```

---

### ✅ Verification (`verification/` - 7 scripts)

Verification and validation:
- Verify insights generation
- Dashboard data verification
- Pipeline verification
- SBOM verification

**Key Scripts:**
- `verify_insights_complete.sh` - Verify insights are created
- `verify_dashboard_data.sh` - Verify dashboard shows correct data
- `verify-e2e-pipeline.sh` - Verify complete pipeline
- `quick_verify_sbom_fix.sh` - Quick SBOM verification

**Usage:**
```bash
cd verification
./verify_insights_complete.sh
```

---

## 🧩 Component Tests (in testing root)

### Admission Webhook & Policy Engine
- `test_admission_webhook.sh` - Test admission webhook
- `test_admission_metrics.sh` - Test webhook metrics
- `test_policy_engine_e2e.sh` - Policy engine E2E test
- `test_policy_api_simple.sh` - Simple policy API test
- `test_webhook_with_pod.sh` - Test webhook with pod creation
- `test-webhook-complete.sh` - Complete webhook test
- `test-webhook-deployment.sh` - Webhook deployment test

**Usage:**
```bash
./test_admission_webhook.sh
./test_policy_engine_e2e.sh
```

---

### Risk Engine & Scoring
- `test_risk_engine.sh` - Test risk engine
- `test_mvp2_phase1_risk_scoring.sh` - MVP2 risk scoring
- `test-v2-scorer.sh` - V2 scorer test
- `test-v2-scorer-complete.sh` - Complete V2 scorer test
- `test-v2-complete.sh` - V2 complete test

**Usage:**
```bash
./test_risk_engine.sh
./test-v2-scorer.sh
```

---

### SBOM & CVE Detection
- `test-cve-detection.sh` - CVE detection test
- `test-sbom-generation.sh` - SBOM generation test
- `trigger-sbom-processing.sh` - Trigger SBOM processing

**Usage:**
```bash
./test-cve-detection.sh
./test-sbom-generation.sh
```

---

### Dashboard Tests
- `test_dashboard.sh` - General dashboard test
- `test_dashboard_comprehensive.sh` - Comprehensive dashboard test
- `test_dashboard_browser.sh` - Browser-based test
- `test_dashboard_screens.sh` - Test all dashboard screens
- `test_dashboard_insight_display.sh` - Test insight display
- `test_dashboard_insight_fix.sh` - Test insight fixes
- `test_all_dashboard_apis.sh` - Test all dashboard APIs
- `test_agent_sync_dashboard.sh` - Test agent sync
- `test_cors_complete.sh` - CORS testing
- `test_login_cors.sh` - Login CORS test

**Usage:**
```bash
./test_dashboard_comprehensive.sh
./test_all_dashboard_apis.sh
```

---

### mTLS Tests
- `test_mtls_connection.sh` - Test mTLS connection
- `test_mtls_traffic_encryption.sh` - Test traffic encryption
- `test_mtls_traffic_from_agent.sh` - Test agent mTLS
- `test_mtls_traffic_with_debug_pod.sh` - Debug pod mTLS
- `demo_mtls_data_transmission.sh` - mTLS demo
- `test_issue5_mtls_advanced.sh` - Advanced mTLS test

**Usage:**
```bash
./test_mtls_connection.sh
./demo_mtls_data_transmission.sh
```

---

### Test Runners
- `run_all_tests.sh` - Run all tests
- `run_tests.sh` - Run specific tests
- `run_failed_tests.sh` - Re-run failed tests
- `run_validation_tests.sh` - Run validation tests
- `run-pending-testcases.sh` - Run pending test cases
- `test_cases.sh` - Test case runner
- `test_all_issues.sh` - Test all known issues

**Usage:**
```bash
./run_all_tests.sh
./run_validation_tests.sh
```

---

### Use Cases
- `usecase_vulnerable_pod.sh` - Vulnerable pod use case
- `usecase_vulnerable_pod_complete.sh` - Complete vulnerable pod scenario

**Usage:**
```bash
./usecase_vulnerable_pod_complete.sh
```

---

### Specific Feature Tests
- `test_issue1_cel_hotreload.sh` - CEL hot reload test
- `test_yaml_rules.sh` - YAML rules test
- `test_yaml_rules_integration.sh` - YAML rules integration
- `test_phase1_components.sh` - Phase 1 components
- `test_insight_status_flow.sh` - Insight status flow
- `test_metrics_with_traffic.sh` - Metrics with traffic
- `test_all_metrics.sh` - All metrics test

**Usage:**
```bash
./test_yaml_rules.sh
./test_all_metrics.sh
```

---

## 🔄 Testing Workflows

### Complete Test Suite
```bash
# 1. Run all tests
./run_all_tests.sh

# 2. If failures, check specific category
cd e2e
./test_e2e_comprehensive.sh

# 3. Verify results
cd ../verification
./verify_insights_complete.sh
```

### Quick Validation
```bash
# Run validation tests
./run_validation_tests.sh

# Verify pipeline
cd verification
./verify-e2e-pipeline.sh
```

### After Code Changes
```bash
# 1. Rebuild
../deployment/rebuild_and_deploy.sh

# 2. Run relevant tests
./test_risk_engine.sh
./test-sbom-generation.sh

# 3. Run E2E
cd e2e
./test_e2e_comprehensive.sh
```

---

## 📊 Test Coverage

### By Component
- **Policy Engine**: 7 tests
- **Risk Engine**: 5 tests
- **SBOM/CVE**: 8 tests
- **Dashboard**: 10 tests
- **mTLS**: 6 tests
- **API**: 8 tests
- **Pipeline**: 19+ E2E tests

### By Type
- **E2E**: 19 tests (complete workflows)
- **Integration**: 8 tests (component interaction)
- **Performance**: 4 tests (load/stress)
- **Verification**: 7 tests (validation)
- **Component**: 47 tests (specific features)

---

## 🚨 Troubleshooting

### Tests Failing
```bash
# Check system status
../monitoring/monitor_fortuna.sh

# Verify deployment
../setup/verify_deployment.sh

# Check logs
kubectl logs -n fortuna -l app=fortuna-core --tail=100
```

### Timeouts
```bash
# Increase timeout in test script
export TEST_TIMEOUT=300

# Or edit script directly
vim test_e2e_comprehensive.sh
```

### Database Issues
```bash
# Clear and reset database
../database/clear_database.sh
../database/setup_database.sh

# Re-run tests
./run_validation_tests.sh
```

---

## 🎯 Best Practices

### Before Running Tests
1. Verify deployment: `../setup/verify_deployment.sh`
2. Check database: `../database/compare_db_k8s.sh`
3. Monitor system: `../monitoring/monitor_fortuna.sh`

### After Tests
1. Check logs for errors
2. Verify insights created
3. Check database state

### Writing New Tests
1. Use descriptive names (`test_<component>_<feature>.sh`)
2. Add to appropriate subdirectory
3. Include timeout handling
4. Add cleanup steps
5. Update this README

---

## 📖 Related Documentation

- [Development Guide](../../docs/04-development/README.md)
- [Testing Strategy](../../docs/04-development/testing/README.md)
- [Deployment Scripts](../deployment/README.md)
- [Monitoring Scripts](../monitoring/README.md)

---

*Back to [Scripts README](../README.md)*

