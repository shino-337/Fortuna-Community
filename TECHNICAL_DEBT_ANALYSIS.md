# Technical Debt Analysis & Resolution Plan

**Date:** 2025-12-23
**Status:** In Progress
**Related:** LOGIC_FLOW_REFACTOR_IMPLEMENTATION.md

---

## Executive Summary

This document analyzes critical technical debt in the KSAM/Fortuna codebase that violates architectural principles and prevents scaling. The issues stem from architectural drift where Agent components perform intelligence operations that should belong to Core.

**Impact:** Cannot implement Attack Path, multi-cluster support, or SaaS features without resolving these issues.

---

## 1. Proto Definition Issues

### 1.1 Import Path Mismatch

**Problem:**
- Proto files declare: `option go_package = "fortuna/api/proto/agent";`
- Core module imports: `example.com/fortunaproto` (placeholder, never defined)
- Agent module: `github.com/ksam/agent`
- Core module: `github.com/ksam/core`

**Files Affected:**
- `api/proto/agent/service.proto`
- `api/proto/agent/sbom.proto`
- `api/proto/agent/cve.proto`
- `agent/internal/client/grpc_client_mtls.go:17`
- `core/go.mod` (line with `example.com/fortunaproto`)

**Resolution:**
```
1. Create shared proto module OR use workspace
2. Update go_package to match actual import path
3. Update all imports to use correct path
4. Regenerate proto files
```

### 1.2 Architecture Violation in Proto

**Problem:**
Proto defines messages that violate Agent/Core separation:

```protobuf
// ❌ WRONG: Agent should NOT send CVE findings
message CVEFinding {
  repeated CVEMatch matches = 9;
  int32 critical_count = 14;
  // ...
}

// ❌ WRONG: Combined finding mixes responsibilities
message CombinedFinding {
  SBOMFinding sbom = 1;
  CVEFinding cve = 2;  // Agent should NOT generate this
}
```

**Why It's Wrong:**
According to LOGIC_FLOW_REFACTOR_IMPLEMENTATION.md:
- Agent = Data Plane (collect SBOM only)
- Core = Control Plane (CVE matching, risk scoring)

**Resolution:**
```
Phase 1: Mark CVEFinding and CombinedFinding as deprecated
Phase 2: Remove from proto after migration complete
```

---

## 2. Multiple Conflicting Client Implementations

### 2.1 Three Different Clients

**Files:**
1. `agent/internal/client/grpc_client.go` (333 lines)
   - Uses HTTP REST, NOT gRPC
   - Sends K8s resources (ServiceAccounts, Roles, etc.)
   - No SBOM support
   - Status: Legacy

2. `agent/internal/client/grpc_client_mtls.go` (195 lines)
   - Uses actual gRPC with mTLS
   - Imports wrong proto path `example.com/fortunaproto`
   - Implements SBOM sending
   - Status: Partially correct

3. `agent/internal/client/grpc_client_combined.go` (46 lines)
   - Tries to use `Client` struct (doesn't exist)
   - Sends `CombinedFinding` (violates architecture)
   - Status: Broken

**Conflicts:**

| Feature | grpc_client.go | grpc_client_mtls.go | grpc_client_combined.go |
|---------|----------------|---------------------|-------------------------|
| Protocol | HTTP REST | gRPC + mTLS | gRPC (broken) |
| Sends SBOM | ❌ No | ✅ Yes | ✅ Yes |
| Sends CVE | ❌ No | ❌ No | ❌ Yes (wrong!) |
| Import Path | Internal types | Wrong proto | Wrong proto |
| TLS Support | ❌ No | ✅ mTLS | ? |

**Resolution:**
```
1. Delete grpc_client.go (legacy HTTP)
2. Delete grpc_client_combined.go (architectural violation)
3. Fix grpc_client_mtls.go import path
4. Rename grpc_client_mtls.go → grpc_client.go
```

---

## 3. Config Mismatch

### 3.1 Agent Config

**File:** `agent/internal/config/config.go`

```go
type Config struct {
    CoreGRPCEndpoint string  // ✅ Correct
    TLSEnabled       bool    // ✅ Correct
    TLSCertPath      string  // ✅ Correct
    TLSKeyPath       string  // ✅ Correct
    TLSCACertPath    string  // ✅ Correct
    BatchSize        int     // ✅ Correct
    // ...
}
```

**Status:** Config structure is correct, but environment variable naming needs consistency.

### 3.2 Core Config

**File:** `core/internal/config/config.go`

```go
type Config struct {
    DatabaseURL      string  // ✅ Correct
    GRPCPort         string  // ✅ Correct
    TLSEnabled       bool    // ✅ Correct
    TLSCertPath      string  // ⚠️  Path differs from Agent expectation
    TLSKeyPath       string  // ⚠️  Path differs from Agent expectation
    TLSCACertPath    string  // ⚠️  Path differs from Agent expectation
    // ...
}
```

**Issues:**
- Agent expects cert at: `/etc/fortuna/tls/client/tls.crt`
- Core expects cert at: `/etc/ksam/certs/tls.crt`
- Namespace mismatch: `fortuna` vs `ksam`

**Resolution:**
```
1. Standardize on namespace: fortuna
2. Agent paths: /etc/fortuna/tls/client/*
3. Core paths: /etc/fortuna/tls/server/*
4. Update deployment manifests
```

---

## 4. Architecture Drift: Agent Doing CVE Matching

### 4.1 Current Flow (WRONG)

```
Pod Created
↓
Agent:
  1. Extract SBOM ✅
  2. Match CVE ❌ (should be in Core)
  3. Calculate risk ❌ (should be in Core)
  4. Send combined data ❌
↓
Core:
  1. Store data ✅
  2. Display in dashboard ✅
```

### 4.2 Correct Flow (TO-BE)

```
Pod Created
↓
Agent:
  1. Extract SBOM ✅
  2. Send SBOM to Core ✅
↓
Core:
  1. Deduplicate by image digest
  2. Match CVE (async worker)
  3. Calculate risk
  4. Create insights
  5. Update graph
```

### 4.3 Code Impact

**Files to Clean Up:**
- Remove any CVE matching logic from `agent/`
- Remove any risk scoring from `agent/`
- Ensure Agent only does SBOM extraction

---

## 5. Missing Proto Module Setup

### 5.1 Current State

```
KSAM/
├── api/
│   └── proto/
│       └── agent/
│           ├── *.proto
│           └── *.pb.go (generated)
├── agent/
│   └── go.mod (github.com/ksam/agent)
└── core/
    └── go.mod (github.com/ksam/core, imports example.com/fortunaproto ❌)
```

### 5.2 Desired State (Option A: Workspace)

```
KSAM/
├── go.work (workspace root)
├── api/
│   ├── go.mod (github.com/ksam/api)
│   └── proto/
├── agent/
│   └── go.mod (imports github.com/ksam/api)
└── core/
    └── go.mod (imports github.com/ksam/api)
```

### 5.3 Desired State (Option B: Monorepo)

```
KSAM/
├── go.mod (github.com/ksam)
└── (all code in same module)
```

**Recommendation:** Option A (Workspace) - maintains separation while sharing proto

---

## 6. Implementation Plan

### Phase 1: Proto Cleanup (Priority: CRITICAL)

- [ ] Create `api/go.mod` with module `github.com/ksam/api`
- [ ] Update proto `go_package` to `github.com/ksam/api/proto/agent`
- [ ] Create workspace `go.work` at root
- [ ] Add all modules to workspace
- [ ] Regenerate proto files
- [ ] Update all imports
- [ ] Remove `example.com/fortunaproto` dependency

### Phase 2: Client Consolidation (Priority: HIGH)

- [ ] Fix imports in `grpc_client_mtls.go`
- [ ] Test mTLS client functionality
- [ ] Delete `grpc_client.go`
- [ ] Delete `grpc_client_combined.go`
- [ ] Rename `grpc_client_mtls.go` → `grpc_client.go`

### Phase 3: Config Alignment (Priority: MEDIUM)

- [ ] Standardize namespace to `fortuna`
- [ ] Update config defaults
- [ ] Update deployment manifests
- [ ] Document environment variables

### Phase 4: Architecture Compliance (Priority: HIGH)

- [ ] Audit Agent code for CVE matching
- [ ] Remove CVE matching from Agent
- [ ] Deprecate CVEFinding in proto
- [ ] Deprecate CombinedFinding in proto
- [ ] Update Agent to send SBOM only

### Phase 5: Documentation (Priority: MEDIUM)

- [ ] Update architecture docs
- [ ] Create migration guide
- [ ] Update deployment guide
- [ ] Create ADR for proto module

---

## 7. Testing Strategy

### 7.1 Unit Tests
- Test proto import in both agent and core
- Test client connection with mTLS
- Test SBOM serialization/deserialization

### 7.2 Integration Tests
- Deploy agent + core with mTLS
- Send SBOM from agent
- Verify Core receives and stores SBOM
- Verify no CVE data sent from agent

### 7.3 Regression Tests
- Ensure existing functionality not broken
- Test backward compatibility during migration

---

## 8. Risks & Mitigation

### 8.1 Breaking Changes

**Risk:** Proto changes break existing deployments
**Mitigation:**
- Keep deprecated fields during transition
- Version proto messages (schema_version field)
- Rolling deployment strategy

### 8.2 Import Path Changes

**Risk:** Compilation failures across modules
**Mitigation:**
- Use workspace for atomic changes
- Test build before committing

### 8.3 mTLS Certificate Issues

**Risk:** Connection failures in production
**Mitigation:**
- Test with both TLS enabled and disabled
- Provide clear certificate setup guide
- Include certificate generation scripts

---

## 9. Success Criteria

- [ ] All modules compile without errors
- [ ] No `example.com/fortunaproto` references
- [ ] Single gRPC client implementation
- [ ] Agent sends SBOM only (no CVE)
- [ ] Core receives SBOM via gRPC
- [ ] mTLS connection works
- [ ] Config values consistent
- [ ] Documentation updated

---

## 10. Next Steps

1. Get approval on workspace approach
2. Create api/go.mod and go.work
3. Fix proto imports
4. Consolidate clients
5. Test end-to-end
6. Update documentation

---

## Appendix A: File Inventory

### Files to Delete
- `agent/internal/client/grpc_client.go` (legacy)
- `agent/internal/client/grpc_client_combined.go` (broken)

### Files to Modify
- `agent/internal/client/grpc_client_mtls.go` (fix imports, rename)
- `agent/internal/config/config.go` (align paths)
- `core/internal/config/config.go` (align paths)
- `api/proto/agent/*.proto` (fix go_package)
- `agent/go.mod` (update dependencies)
- `core/go.mod` (update dependencies)

### Files to Create
- `api/go.mod` (new proto module)
- `go.work` (workspace file)
- Migration guide document
- mTLS setup guide

---

## Appendix B: Proto Module Options Comparison

| Aspect | Workspace | Monorepo | Separate Repo |
|--------|-----------|----------|---------------|
| Complexity | Low | Very Low | High |
| Versioning | Per-module | Single | Independent |
| Build Speed | Fast | Fastest | Slow |
| Separation | Good | None | Excellent |
| Recommended | ✅ Yes | ⚠️ Maybe | ❌ No |

**Decision:** Use Workspace approach for balance of simplicity and modularity.
