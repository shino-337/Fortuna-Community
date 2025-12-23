# Testing Documentation

This directory contains documentation about testing strategies, test readiness, and quality assurance.

---

## 📚 Documents

### [TEST_READINESS_REPORT.md](./TEST_READINESS_REPORT.md)
**Comprehensive test readiness assessment**
- Test coverage by component
- Test infrastructure status
- E2E test scenarios
- Known issues and gaps

---

## 🧪 Testing Strategy

### Test Pyramid

```
        ┌─────────────┐
        │  E2E Tests  │ ← Few, Slow, Expensive
        ├─────────────┤
        │ Integration │ ← Moderate, Medium Cost
        ├─────────────┤
        │ Unit Tests  │ ← Many, Fast, Cheap
        └─────────────┘
```

### Coverage Goals

| Level | Target | Current | Status |
|-------|--------|---------|--------|
| **Unit** | 80% | 60% | 🟡 In Progress |
| **Integration** | 70% | 40% | 🟡 In Progress |
| **E2E** | 90% | 75% | 🟢 Good |

---

## 📝 Test Types

### 1. Unit Tests

**Location**: `KSAM/core/pkg/*_test.go`, `KSAM/agent/*_test.go`

**Coverage**:
- Individual functions/methods
- Edge cases
- Error handling
- Mocking external dependencies

**Example**:
```go
func TestSBOMExtractor_ParseDebianStatus(t *testing.T) {
    extractor := NewSBOMExtractor()
    packages, err := extractor.ParseDebianStatus(testData)
    
    assert.NoError(t, err)
    assert.Equal(t, 5, len(packages))
    assert.Equal(t, "nginx", packages[0].Name)
}
```

**Run**:
```bash
cd KSAM/core
go test ./... -v -cover
```

---

### 2. Integration Tests

**Location**: `KSAM/core/pkg/*_integration_test.go`

**Coverage**:
- Component interactions
- Database operations
- NATS messaging
- gRPC communication

**Example**:
```go
// +build integration

func TestCVEMatcher_MatchWithDatabase(t *testing.T) {
    // Requires real PostgreSQL
    db := setupTestDatabase(t)
    defer db.Close()
    
    matcher := NewCVEMatcher(db)
    matches, err := matcher.Match(testSBOM)
    
    assert.NoError(t, err)
    assert.Greater(t, len(matches), 0)
}
```

**Run**:
```bash
cd KSAM/core
go test -tags=integration ./... -v
```

---

### 3. End-to-End Tests

**Location**: `KSAM/tests/e2e/*.sh`

**Coverage**:
- Complete user workflows
- Multi-component scenarios
- Real Kubernetes environment
- Actual CVE detection

**Tests**:
- CVE insights generation
- SBOM caching
- Version boundary matching
- Multi-container pods
- Pipeline verification

**Run**:
```bash
cd KSAM/tests/e2e
./test_e2e_cve_insights_v2.sh
```

---

## 🛠️ Test Infrastructure

### Local Development

**Prerequisites**:
- Go 1.23+
- Docker
- Minikube or kind
- PostgreSQL (for integration tests)

**Setup**:
```bash
# Install test dependencies
go install gotest.tools/gotestsum@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Start test environment
./scripts/setup_test_env.sh
```

### CI/CD

**GitHub Actions** (`.github/workflows/test.yml`):
```yaml
name: Tests
on: [push, pull_request]

jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test ./... -v -cover

  integration:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        ports:
          - 5432:5432
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: go test -tags=integration ./... -v

  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: helm/kind-action@v1
      - run: |
          cd tests/e2e
          ./test_e2e_cve_insights_v2.sh
```

---

## 📊 Test Metrics

### Current State

**Unit Tests**:
- Total: 250+ tests
- Coverage: 60%
- Execution time: ~30 seconds

**Integration Tests**:
- Total: 50+ tests
- Coverage: 40%
- Execution time: ~2 minutes

**E2E Tests**:
- Total: 5 tests
- Coverage: 75% of user flows
- Execution time: ~10 minutes

### Quality Gates

**Pre-merge Requirements**:
- ✅ All unit tests pass
- ✅ No linter errors
- ✅ Coverage ≥ 60% for changed files
- ✅ Integration tests pass
- ⚠️  E2E tests pass (manual for now)

---

## 🐛 Debugging Tests

### Failed Unit Test
```bash
# Run specific test
go test -v -run TestName ./pkg/component

# With verbose output
go test -v -run TestName ./pkg/component -args -test.v

# With race detector
go test -race -run TestName ./pkg/component
```

### Failed Integration Test
```bash
# Check database
psql -U postgres -d fortuna_test -c "SELECT * FROM cves LIMIT 5;"

# Check logs
tail -f /tmp/fortuna-test.log
```

### Failed E2E Test
```bash
# Check pod logs
kubectl logs -n fortuna -l app=fortuna-core --tail=100

# Check events
kubectl get events -n fortuna --sort-by='.lastTimestamp'

# Debug pod
kubectl exec -it -n fortuna fortuna-core-xxx -- /bin/sh
```

---

## 📝 Writing Tests

### Best Practices

1. **Descriptive Names**: `TestSBOMExtractor_ParseDebianStatus_WithValidInput`
2. **Arrange-Act-Assert**: Structure tests clearly
3. **Isolated**: No dependencies between tests
4. **Fast**: Keep unit tests < 100ms
5. **Deterministic**: No flaky tests

### Test Template

```go
func TestComponent_Method_Scenario(t *testing.T) {
    // Arrange
    input := prepareTestData()
    expected := expectedResult()
    component := NewComponent()
    
    // Act
    actual, err := component.Method(input)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected, actual)
}
```

### Table-Driven Tests

```go
func TestVersionMatching(t *testing.T) {
    tests := []struct {
        name     string
        version  string
        range    string
        expected bool
    }{
        {"exact match", "1.0.0", "1.0.0", true},
        {"in range", "1.5.0", ">=1.0.0,<2.0.0", true},
        {"out of range", "2.5.0", ">=1.0.0,<2.0.0", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            actual := MatchVersion(tt.version, tt.range)
            assert.Equal(t, tt.expected, actual)
        })
    }
}
```

---

## 🚀 Continuous Improvement

### Test Coverage Goals (Q1 2025)

- [ ] Unit tests: 60% → 80%
- [ ] Integration tests: 40% → 70%
- [ ] E2E tests: 75% → 90%
- [ ] Performance tests: 0% → 50%

### Planned Improvements

- [ ] Automated E2E testing in CI
- [ ] Performance regression tests
- [ ] Chaos engineering tests
- [ ] Security testing (SAST/DAST)
- [ ] Load testing for scalability

---

## 📚 Related Documentation

- [Test Suite Overview](../../../tests/README.md)
- [Development Guide](../README.md)
- [CI/CD Pipeline](../CICD.md)
- [Contributing Guide](../CONTRIBUTING.md)

---

**Questions?** Open an issue or reach out in Slack.

**Last Updated**: December 2024 (Fortuna v2.0)

