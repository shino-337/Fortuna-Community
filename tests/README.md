# Fortuna Testing Suite

This directory contains all test scripts and test-related documentation for Fortuna K8s Management Platform.

---

## 📂 Directory Structure

```
tests/
├── e2e/              ← End-to-end tests
├── integration/      ← Integration tests
└── unit/             ← Unit tests (in component code)
```

---

## 🧪 Test Types

### End-to-End Tests (`e2e/`)

**Purpose**: Verify complete user workflows from deployment to verification.

**Tests**:
- `test_e2e_cve_insights_v2.sh` - CVE detection and insights generation
- `test_e2e_sbom_cache_hit.sh` - SBOM caching and reuse
- `test_e2e_cve_boundary_versions.sh` - CVE version boundary matching
- `test_e2e_multi_container_sbom_cve.sh` - Multi-container pod handling
- `verify_pipeline_logic.sh` - Pipeline flow verification

**How to Run**:
```bash
cd tests/e2e
./test_e2e_cve_insights_v2.sh
```

### Integration Tests (`integration/`)

**Purpose**: Test interactions between components (Core ↔ Agent, Core ↔ PostgreSQL, etc.)

**Status**: Coming soon

### Unit Tests (`unit/`)

**Purpose**: Unit tests are embedded in component code (`*_test.go` files)

**How to Run**:
```bash
cd KSAM/core
go test ./...
```

---

## ⚙️ Test Environment Setup

### Prerequisites
- Kubernetes cluster (Minikube recommended for testing)
- kubectl configured
- Fortuna deployed
- PostgreSQL with test data

### Setup Script
```bash
# From repo root
./scripts/setup_test_env.sh
```

---

## 📊 Test Coverage

| Component | Unit Tests | Integration | E2E | Coverage |
|-----------|-----------|-------------|-----|----------|
| **Core** | ✅ 60% | ⏳ Planned | ✅ Yes | 70% |
| **Agent** | ✅ 40% | ⏳ Planned | ✅ Yes | 50% |
| **SBOM** | ✅ 70% | ✅ Yes | ✅ Yes | 80% |
| **CVE** | ✅ 65% | ✅ Yes | ✅ Yes | 75% |
| **Risk** | ✅ 50% | ⏳ Planned | ✅ Yes | 60% |
| **Policy** | ✅ 55% | ⏳ Planned | ⏳ Planned | 55% |

**Target**: 80% code coverage across all components

---

## 🚀 Running Tests

### Quick Test
```bash
# Run single E2E test
cd tests/e2e
./test_e2e_cve_insights_v2.sh
```

### Full Test Suite
```bash
# Run all E2E tests
cd tests/e2e
for test in test_*.sh; do
  echo "Running $test..."
  ./"$test" || echo "❌ $test failed"
done
```

### CI/CD Integration
```bash
# Run in CI
make test-e2e
```

---

## 📝 Writing New Tests

### E2E Test Template

```bash
#!/bin/bash
set -euo pipefail

# Test: <Test Name>
# Purpose: <What this test verifies>

# Setup
kubectl create namespace test-$$
trap "kubectl delete namespace test-$$" EXIT

# Test steps
echo "1. Deploy test resources..."
kubectl apply -f test-resources.yaml -n test-$$

echo "2. Wait for processing..."
sleep 30

echo "3. Verify results..."
RESULT=$(kubectl get insights -n test-$$ -o json)

if [[ "$RESULT" =~ "expected_value" ]]; then
  echo "✅ Test passed"
  exit 0
else
  echo "❌ Test failed"
  exit 1
fi
```

### Test Checklist
- [ ] Clear test name and purpose
- [ ] Cleanup (trap or defer)
- [ ] Isolated test namespace
- [ ] Descriptive output
- [ ] Exit codes (0 = pass, 1 = fail)
- [ ] Documentation

---

## 🐛 Debugging Failed Tests

### Check Logs
```bash
# Core logs
kubectl logs -n fortuna -l app=fortuna-core --tail=100

# Agent logs
kubectl logs -n fortuna -l app=fortuna-agent --tail=100

# Database
kubectl exec -n fortuna postgres-xxx -- \
  psql -U postgres -d fortuna -c "SELECT * FROM insights ORDER BY id DESC LIMIT 10;"
```

### Common Issues

**Test timeout**:
- Increase wait time in test script
- Check if NATS/PostgreSQL are healthy

**Resources not found**:
- Verify namespace exists
- Check RBAC permissions

**Assertion failures**:
- Review expected vs actual output
- Check timing (race conditions)

---

## 📈 Test Metrics

### Performance Benchmarks
- SBOM generation: <2 seconds target
- CVE matching: <1 second target
- Insight creation: <100ms target

### Reliability Targets
- E2E tests: 100% pass rate
- Flakiness: <1% (tests should be deterministic)

---

## 🔗 Related Documentation

- [Development Guide](../docs/development/README.md)
- [CI/CD Pipeline](../docs/development/CICD.md)
- [Test Readiness Report](../docs/development/testing/TEST_READINESS_REPORT.md)

---

**Questions?** Check [Developer FAQ](../docs/development/FAQ.md) or open an issue.

