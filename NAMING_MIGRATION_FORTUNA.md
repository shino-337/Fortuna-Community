# KSAM → Fortuna Naming Migration Analysis

**Date:** 2025-12-23
**Status:** Incomplete Migration Detected
**Priority:** CRITICAL - Blocking all development

---

## Executive Summary

The project is in the middle of a naming migration from **KSAM** (Kubernetes Service Account Management) to **Fortuna**, but the migration is incomplete. This creates significant technical debt:

- ❌ Go modules still use `github.com/ksam/*`
- ❌ Proto uses placeholder `example.com/fortunaproto`
- ✅ Proto package names already use `fortuna.agent.v1`
- ✅ Kubernetes resources already use `fortuna-*` naming
- ⚠️ Config paths mix `/etc/ksam/` and `/etc/fortuna/`

**Impact:** Cannot build or deploy consistently until naming is unified.

---

## 1. Current State Analysis

### 1.1 Go Module Names (INCONSISTENT)

| Component | Current Module Path | Should Be |
|-----------|-------------------|-----------|
| Agent | `github.com/ksam/agent` | `github.com/fortuna/agent` |
| Core | `github.com/ksam/core` | `github.com/fortuna/core` |
| API/Proto | `example.com/fortunaproto` | `github.com/fortuna/api` |

**Files Affected:**
- `agent/go.mod:1` → `module github.com/ksam/agent`
- `core/go.mod:1` → `module github.com/ksam/core`
- `api/proto/agent/go.mod:1` → `module example.com/fortunaproto`

### 1.2 Proto Package Names (CORRECT ✅)

```protobuf
// All proto files already use fortuna
package fortuna.agent.v1;

option go_package = "fortuna/api/proto/agent";  // ❌ Path is wrong
```

**Status:** Package name is correct, but `go_package` path doesn't match actual import.

### 1.3 Import Paths (BROKEN 💥)

**Current imports in code:**
```go
// 13 files import this:
import pb "example.com/fortunaproto"

// But go.mod has:
replace example.com/fortunaproto => ../api/proto/agent
```

**Problem:**
- `example.com/fortunaproto` is a placeholder, not a real module
- Relies on local `replace` directive
- Cannot be used by external packages
- Not suitable for deployment

### 1.4 Kubernetes Resources (CORRECT ✅)

```yaml
# deploy/fortuna-core-deployment.yaml
# deploy/fortuna-agent-daemonset.yaml
# deploy/fortuna-rbac.yaml
```

**Status:** Already using `fortuna` namespace and resource names.

### 1.5 Configuration Paths (MIXED ⚠️)

**Agent Config** (`agent/internal/config/config.go:48-52`):
```go
CoreGRPCEndpoint: "fortuna-core.fortuna.svc.cluster.local:9090"  // ✅
TLSCertPath:      "/etc/fortuna/tls/client/tls.crt"              // ✅
TLSKeyPath:       "/etc/fortuna/tls/client/tls.key"              // ✅
TLSCACertPath:    "/etc/fortuna/tls/client/ca.crt"               // ✅
```

**Core Config** (`core/internal/config/config.go:60-62`):
```go
TLSCACertPath:   "/etc/ksam/ca-cert/ca.crt"      // ❌ Still KSAM
TLSCertPath:     "/etc/ksam/certs/tls.crt"       // ❌ Still KSAM
TLSKeyPath:      "/etc/ksam/certs/tls.key"       // ❌ Still KSAM
```

**Database** (`core/internal/config/config.go:50`):
```go
DatabaseURL: "postgres://postgres:postgres@postgres:5432/ksam?sslmode=disable"  // ❌ Still KSAM
```

**NATS** (`core/internal/config/config.go:54`):
```go
NATSEndpoint: "nats://nats.ksam.svc.cluster.local:4222"  // ❌ Still KSAM
```

### 1.6 Code References

**Files containing "ksam":** 87 files
**Files containing "fortuna":** 15 files

**Key files mixing both:**
- Most Go files use `github.com/ksam/*` imports
- Proto-related files use `example.com/fortunaproto`
- Config files use mix of `/etc/ksam/` and `/etc/fortuna/`

---

## 2. Why This Matters

### 2.1 Build Issues
- Proto generation creates wrong import paths
- Module resolution relies on `replace` directives
- Cannot publish to registry
- Breaks CI/CD pipelines

### 2.2 Deployment Issues
- TLS certs mounted at `/etc/ksam/` won't be found by Agent (expects `/etc/fortuna/`)
- Database name mismatch
- Service discovery name mismatch

### 2.3 Developer Experience
- Confusing which name to use
- Documentation inconsistency
- Hard to onboard new developers

---

## 3. Migration Strategy

### Phase 1: Update Go Modules (CRITICAL)

**Step 1.1:** Update `api/go.mod`
```go
module github.com/fortuna/api

go 1.24.0

require (
	google.golang.org/grpc v1.77.0
	google.golang.org/protobuf v1.36.11
)
```

**Step 1.2:** Update `agent/go.mod`
```go
module github.com/fortuna/agent

go 1.24.0

require (
	github.com/fortuna/api v0.0.0
	// ... other deps
)
```

**Step 1.3:** Update `core/go.mod`
```go
module github.com/fortuna/core

go 1.24.0

require (
	github.com/fortuna/api v0.0.0
	// ... other deps
)
```

**Step 1.4:** Update `go.work`
```go
go 1.24.0

use (
	./api
	./agent
	./core
)
```

### Phase 2: Update Proto Files

**Step 2.1:** Update all `.proto` files
```protobuf
package fortuna.agent.v1;

option go_package = "github.com/fortuna/api/proto/agent";
```

**Step 2.2:** Regenerate proto files
```bash
cd api
buf generate
# or
protoc --go_out=. --go-grpc_out=. proto/agent/*.proto
```

### Phase 3: Update All Imports

**Step 3.1:** Replace in all `.go` files:
```bash
# Find and replace
FROM: import pb "example.com/fortunaproto"
TO:   import pb "github.com/fortuna/api/proto/agent"

FROM: import fortuna "example.com/fortunaproto"
TO:   import fortuna "github.com/fortuna/api/proto/agent"

FROM: "github.com/ksam/agent/
TO:   "github.com/fortuna/agent/

FROM: "github.com/ksam/core/
TO:   "github.com/fortuna/core/
```

**Affected files:** ~100+ Go files

### Phase 4: Update Configuration Paths

**Step 4.1:** Update Core config defaults (`core/internal/config/config.go`):
```go
TLSCACertPath: "/etc/fortuna/tls/server/ca.crt"
TLSCertPath:   "/etc/fortuna/tls/server/tls.crt"
TLSKeyPath:    "/etc/fortuna/tls/server/tls.key"
DatabaseURL:   "postgres://postgres:postgres@postgres:5432/fortuna?sslmode=disable"
NATSEndpoint:  "nats://nats.fortuna.svc.cluster.local:4222"
```

**Step 4.2:** Update deployment manifests:
- Volume mount paths
- Environment variables
- Service names
- Namespace references

### Phase 5: Update Documentation

**Update all docs:**
- Replace "KSAM" → "Fortuna" in markdown files
- Update diagrams
- Update README
- Update API references

---

## 4. Implementation Checklist

### 4.1 Pre-Migration

- [x] Create this analysis document
- [ ] Backup current working state
- [ ] Review all import paths
- [ ] Identify external dependencies

### 4.2 Module Migration

- [ ] Update `api/go.mod` to `github.com/fortuna/api`
- [ ] Update `agent/go.mod` to `github.com/fortuna/agent`
- [ ] Update `core/go.mod` to `github.com/fortuna/core`
- [ ] Update `go.work` with new paths
- [ ] Remove `example.com/fortunaproto` replace directives

### 4.3 Proto Migration

- [ ] Update `go_package` in all `.proto` files
- [ ] Regenerate all `.pb.go` files
- [ ] Verify generated code correctness

### 4.4 Import Updates

- [ ] Replace `example.com/fortunaproto` imports (13 files)
- [ ] Replace `github.com/ksam/agent` imports (~50+ files)
- [ ] Replace `github.com/ksam/core` imports (~50+ files)
- [ ] Verify no broken imports

### 4.5 Config Updates

- [ ] Update Core TLS paths to `/etc/fortuna/tls/server/`
- [ ] Update database name to `fortuna`
- [ ] Update NATS endpoint to `fortuna` namespace
- [ ] Update deployment manifests

### 4.6 Verification

- [ ] Build agent: `cd agent && go build ./cmd/`
- [ ] Build core: `cd core && go build ./cmd/`
- [ ] Generate proto: `cd api && buf generate`
- [ ] Run tests: `go test ./...`
- [ ] Verify no `ksam` references in code
- [ ] Verify no `example.com/fortunaproto` references

### 4.7 Documentation

- [ ] Update README.md
- [ ] Update architecture docs
- [ ] Update deployment guides
- [ ] Update API documentation

---

## 5. Automated Migration Script

```bash
#!/bin/bash
# migrate-to-fortuna.sh

set -e

echo "🔄 Starting KSAM → Fortuna migration..."

# 1. Update go.mod files
echo "📦 Updating Go modules..."
sed -i '' 's|module github.com/ksam/agent|module github.com/fortuna/agent|g' agent/go.mod
sed -i '' 's|module github.com/ksam/core|module github.com/fortuna/core|g' core/go.mod
sed -i '' 's|module example.com/fortunaproto|module github.com/fortuna/api|g' api/go.mod

# 2. Update proto files
echo "🔧 Updating proto files..."
find api/proto -name "*.proto" -exec sed -i '' \
  's|option go_package = "fortuna/api/proto/agent"|option go_package = "github.com/fortuna/api/proto/agent"|g' {} \;

# 3. Update imports in Go files
echo "🔍 Updating imports..."
find . -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" -exec sed -i '' \
  -e 's|github.com/ksam/agent|github.com/fortuna/agent|g' \
  -e 's|github.com/ksam/core|github.com/fortuna/core|g' \
  -e 's|example.com/fortunaproto|github.com/fortuna/api/proto/agent|g' \
  {} \;

# 4. Update config paths
echo "⚙️  Updating config paths..."
sed -i '' 's|/etc/ksam/|/etc/fortuna/tls/server/|g' core/internal/config/config.go
sed -i '' 's|postgres:5432/ksam|postgres:5432/fortuna|g' core/internal/config/config.go
sed -i '' 's|nats.ksam.svc|nats.fortuna.svc|g' core/internal/config/config.go

# 5. Remove replace directives
echo "🗑️  Removing temporary replace directives..."
sed -i '' '/replace example.com\/fortunaproto/d' agent/go.mod core/go.mod

# 6. Regenerate proto
echo "🔨 Regenerating proto files..."
cd api
buf generate || protoc --go_out=. --go-grpc_out=. proto/agent/*.proto
cd ..

# 7. Tidy modules
echo "🧹 Tidying modules..."
cd agent && go mod tidy && cd ..
cd core && go mod tidy && cd ..
cd api && go mod tidy && cd ..

# 8. Build verification
echo "✅ Verifying builds..."
cd agent && go build ./cmd/ && cd ..
cd core && go build ./cmd/ && cd ..

echo "🎉 Migration complete!"
echo "⚠️  Remember to:"
echo "  - Update deployment manifests"
echo "  - Update documentation"
echo "  - Recreate database with name 'fortuna'"
echo "  - Update Kubernetes resources"
```

---

## 6. Rollback Plan

If migration fails:

```bash
# Revert all changes
git checkout agent/go.mod core/go.mod api/go.mod go.work
git checkout agent/ core/ api/
go mod tidy
```

---

## 7. Post-Migration Validation

### 7.1 Code Validation
```bash
# No references to old names
! grep -r "github.com/ksam" . --include="*.go" | grep -v "vendor/"
! grep -r "example.com/fortunaproto" . --include="*.go" | grep -v "vendor/"
! grep -r "/etc/ksam/" . --include="*.go"

# All modules use fortuna
grep "module github.com/fortuna" agent/go.mod
grep "module github.com/fortuna" core/go.mod
grep "module github.com/fortuna" api/go.mod
```

### 7.2 Build Validation
```bash
cd agent && go build ./... && cd ..
cd core && go build ./... && cd ..
cd api && go build ./... && cd ..
```

### 7.3 Proto Validation
```bash
# Check generated files use correct import
head -20 api/proto/agent/service.pb.go | grep "github.com/fortuna/api"
```

---

## 8. Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Build failures | High | High | Test in isolated branch |
| Import errors | High | High | Use find/replace carefully |
| Deployment breaks | Medium | High | Update manifests together |
| Database migration | Low | High | Script migration separately |
| Lost work | Low | Critical | Git commit before starting |

---

## 9. Timeline Estimate

- **Module updates:** 1 hour
- **Import updates:** 2 hours
- **Config updates:** 1 hour
- **Testing:** 2 hours
- **Documentation:** 2 hours
- **Total:** ~8 hours (1 working day)

---

## 10. Decision: Proceed with Fortuna

**Recommendation:** Complete migration to Fortuna naming now.

**Rationale:**
1. Proto already uses `fortuna` package
2. K8s resources already use `fortuna` names
3. Halfway migration creates technical debt
4. Blocking future development
5. Only affects internal code, not API contracts

**Approval Required:** Yes - affects all modules

---

## Appendix A: File Change Counts

| Change Type | Files Affected | Lines Changed |
|-------------|----------------|---------------|
| go.mod updates | 3 files | ~10 lines |
| Proto updates | 3 files | ~3 lines |
| Import updates | ~100 files | ~200 lines |
| Config updates | 2 files | ~10 lines |
| **Total** | **~108 files** | **~223 lines** |

---

## Appendix B: External Dependencies

**None affected** - All changes are internal module names and paths.

---

## Appendix C: Related Documents

- `TECHNICAL_DEBT_ANALYSIS.md` - Proto import issues
- `LOGIC_FLOW_REFACTOR_IMPLEMENTATION.md` - Architecture guidelines
- `docs/06-reference/migration/MIGRATION_GUIDE_KSAM_TO_FORTUNA.md` - User migration guide
