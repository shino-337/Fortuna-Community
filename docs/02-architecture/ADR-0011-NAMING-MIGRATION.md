# ADR-0011: KSAM to Fortuna Naming Migration

**Status:** Implemented
**Date:** 2025-12-23
**Deciders:** Architecture Team
**Related:** ADR-0010 (Agent Architecture)

---

## Context

The project was originally named "KSAM" (Kubernetes Service Account Management) but evolved into a broader security platform. The naming was inconsistent:

- Go modules: `github.com/ksam/*`
- Proto package: `fortuna.agent.v1`
- K8s resources: `fortuna-*`
- Proto imports: `example.com/fortunaproto` (placeholder)
- Config paths: Mix of `/etc/ksam/` and `/etc/fortuna/`

This inconsistency created:
- Build issues (proto resolution failures)
- Developer confusion
- Deployment complexity
- Technical debt blocking new features

## Decision

**We will complete the migration to "Fortuna" naming across all layers:**

1. **Module Names**
   - `github.com/fortuna/api` - Shared proto definitions
   - `github.com/fortuna/agent` - Agent binary
   - `github.com/fortuna/core` - Core control plane

2. **Import Paths**
   - Proto: `github.com/fortuna/api/proto/agent`
   - No more `example.com/fortunaproto` placeholder

3. **Configuration Paths**
   - Agent: `/etc/fortuna/tls/client/`
   - Core: `/etc/fortuna/tls/server/`
   - Database: `fortuna`
   - K8s namespace: `fortuna`

4. **Workspace Structure**
   - Use Go workspace for local development
   - Replace directives for module resolution
   - Single `go.work` at project root

## Rationale

### Why Fortuna?

1. **Reflects Evolution** - No longer just service account management
2. **Broader Scope** - CVE scanning, SBOM, attack paths, policy enforcement
3. **Market Positioning** - More appealing for security platform
4. **Consistency** - K8s resources already use `fortuna-*`

### Why Now?

1. **Blocking Issues** - Proto import failures prevent development
2. **Technical Debt** - Halfway migration creates complexity
3. **Scale Readiness** - Must fix before multi-cluster/SaaS features
4. **Attack Path** - New features need clean architecture

### Alternatives Considered

#### Alternative 1: Keep KSAM Naming
- **Pros:** No migration effort
- **Cons:** Doesn't reflect current scope, K8s resources already use Fortuna
- **Rejected:** Inconsistency remains

#### Alternative 2: Different Name (e.g., "Sentinel")
- **Pros:** Fresh start
- **Cons:** K8s resources would need changing, breaks existing deployments
- **Rejected:** Too disruptive

#### Alternative 3: Gradual Migration
- **Pros:** Lower risk per change
- **Cons:** Prolongs inconsistency, multiple breaking changes
- **Rejected:** Better to fix once

## Implementation

### Phase 1: Module Structure ✅
1. Created `api/go.mod` with `github.com/fortuna/api`
2. Updated `agent/go.mod` to `github.com/fortuna/agent`
3. Updated `core/go.mod` to `github.com/fortuna/core`
4. Configured `go.work` at project root
5. Added replace directives for local development

### Phase 2: Proto Definitions ✅
1. Updated all `.proto` files with `option go_package = "github.com/fortuna/api/proto/agent"`
2. Regenerated all `.pb.go` and `_grpc.pb.go` files
3. Deleted obsolete `api/proto/agent/go.mod`

### Phase 3: Import Updates ✅
1. Created automated migration script
2. Updated 100+ Go files:
   - `example.com/fortunaproto` → `github.com/fortuna/api/proto/agent`
   - `github.com/ksam/agent` → `github.com/fortuna/agent`
   - `github.com/ksam/core` → `github.com/fortuna/core`

### Phase 4: Configuration (In Progress)
1. ⚠️ Update Core config paths to `/etc/fortuna/`
2. ⚠️ Update database name to `fortuna`
3. ⚠️ Update NATS endpoint to `fortuna` namespace

### Phase 5: Deployment (Pending)
1. Update K8s manifests with new paths
2. Regenerate TLS certificates
3. Database migration script

## Consequences

### Positive

1. **Build Success** - All modules compile, proto imports resolve
2. **Developer Experience** - Single, consistent naming
3. **Deployment Clarity** - Config paths match resource names
4. **Future Proof** - Ready for new features (Attack Path, multi-cluster)
5. **Professional** - Consistent branding across code and docs

### Negative

1. **Breaking Change** - Existing deployments need update
2. **Database Migration** - Must rename database
3. **Certificate Regeneration** - New paths require new certs
4. **Documentation Update** - All docs reference KSAM

### Neutral

1. **One-Time Cost** - Effort spent now prevents future issues
2. **Workspace Setup** - Replace directives needed for local dev
3. **Git History** - Some historical references to KSAM remain

## Risks & Mitigation

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Build failures | Low | High | Tested during implementation ✅ |
| Deployment issues | Medium | High | Update manifests with migration |
| Lost data | Low | Critical | Database migration script + backup |
| Developer confusion | Low | Medium | Clear documentation + ADR |
| External tool breakage | Low | Medium | Search for hardcoded references |

## Compliance

### With Architecture Principles ✅

- **Separation of Concerns** - Proto in separate module
- **Agent as Data Plane** - Naming doesn't affect responsibility split
- **Core as Control Plane** - Module boundaries clear

### With LOGIC_FLOW_REFACTOR ✅

- Migration enables proper Agent/Core separation
- No architectural drift introduced
- Prepares for CVE matching removal from Agent

## Verification

### Build Tests ✅
```bash
cd api && go mod tidy     # Success
cd agent && go mod tidy   # Success
cd core && go mod tidy    # Success
```

### Import Verification ✅
```bash
# No old imports found
! grep -r "example.com/fortunaproto" . --include="*.go"
! grep -r "github.com/ksam" . --include="*.go"
```

### Proto Verification ✅
```bash
# Generated files use correct package
grep "github.com/fortuna/api/proto/agent" api/proto/agent/*.pb.go
```

## Migration Guide

**For Users/Operators:**

See `docs/06-reference/migration/NAMING_MIGRATION_FORTUNA.md` for step-by-step migration.

**For Developers:**

1. Pull latest code
2. Run `go work sync`
3. Rebuild: `cd agent && go build ./cmd/`
4. Update any custom tooling to use new import paths

## References

- `docs/06-reference/technical-debt/TECHNICAL_DEBT_ANALYSIS.md` - Detailed analysis
- `docs/06-reference/migration/NAMING_MIGRATION_FORTUNA.md` - Migration steps
- `docs/04-development/IMPLEMENTATION_COMPLETE_SUMMARY.md` - Implementation status
- `docs/02-architecture/LOGIC_FLOW_REFACTOR_IMPLEMENTATION.md` - Architecture guide

## Notes

- This ADR documents a decision that was implemented proactively to unblock development
- The migration was necessary to resolve import path conflicts preventing builds
- Future ADRs will address Agent/Core responsibility separation (CVE matching removal)

---

**Approved by:** Architecture Team
**Implementation:** 95% complete (config updates remaining)
**Next Review:** After deployment to production
