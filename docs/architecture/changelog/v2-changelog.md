# ARCHITECTURE.md v2.0 Changelog

**Date**: December 15, 2025
**Type**: Major Rewrite (70% changed)
**Old Version**: [Archived](archive/old-architecture/ARCHITECTURE_v1_backup_20251215.md)

## Summary

Comprehensive rewrite of ARCHITECTURE.md to reflect the actual MVP2 implementation state, removing outdated "to be implemented" claims and documenting completed features.

## Major Changes

### 1. Document Structure

**Before**: 3,205 lines, overwhelming detail
**After**: 980 lines, focused and concise

**New Structure**:
- Added Table of Contents
- Reorganized into logical sections
- Removed redundant examples
- Focused on current state vs. future vision

### 2. Status Updates (Critical)

All status markers updated to reflect reality:

| Component | Old Status | New Status | Notes |
|-----------|-----------|------------|-------|
| **NATS JetStream** | 🔧 To be implemented (MVP-1 critical) | ✅ Complete | Fully implemented in MVP1 |
| **Policy Engine** | ⚠️ 20% Complete (skeleton exists) | ✅ Complete (MVP2) | CEL evaluator, enforcement, remediation all done |
| **Admission Webhook** | Not mentioned | ✅ Complete (MVP2) | ValidatingWebhookConfiguration integrated |
| **Apache AGE** | ⏳ To be implemented (MVP-2) | ✅ Complete (MVP2) | Graph queries operational |
| **mTLS** | ⏳ To be implemented (MVP-1 critical) | ✅ Complete | Agent-Core communication secured |
| **Risk Scoring V2** | Not mentioned | ✅ Complete (MVP2) | New algorithm with auto-resolution |
| **Dashboard D3** | Mentioned generically | ✅ Complete (MVP2) | 9 pages, force-directed graph, pink branding |
| **CVE Scanning** | Not mentioned | ⏳ Partial (40%) | Schema ready, Trivy incomplete |
| **SBOM** | Not mentioned | ⏳ Partial (30%) | Extractors done, pipeline incomplete |
| **eBPF Runtime** | High priority, detailed | ⏳ Planned MVP3 | Deprioritized in favor of policy/CVE |

### 3. Added Missing Sections

**New Sections**:
- **Policy Engine** (complete with CEL examples, webhook integration)
- **Risk Scoring V2** (algorithm details, auto-resolution)
- **Admission Webhook** (ValidatingWebhookConfiguration, enforcement modes)
- **CVE & SBOM Integration** (current partial state)
- **Insight Lifecycle** (status transitions, soft delete)
- **Data Flow** (4 flows: inventory, policy, risk, auto-resolution)

### 4. Roadmap Corrections

**Before**:
- MVP-1: 80% complete
- MVP-2: 0% complete
- Unrealistic timelines (weeks for complex features)

**After**:
- MVP-1: ✅ 100% COMPLETE (Released v4.3.0, December 2025)
- MVP-2: ⏳ 60% COMPLETE (Policy, Webhook, Risk V2 done; CVE/SBOM partial)
- MVP-3: 📋 PLANNED (eBPF, ML, advanced features)
- No timeline estimates, just what's done/in-progress/planned

### 5. Technology Stack Updates

**Corrections**:
- Frontend State: Redux Toolkit → **Zustand** (actual implementation)
- Policy Engine: Not mentioned → **Google CEL** (core component)
- Graph: Vague mention → **Apache AGE** (PostgreSQL extension)
- Build Tool: ✅ Vite (confirmed correct)

### 6. Removed/Reduced Content

**Removed**:
- 2,000+ lines of eBPF implementation details (not started, deprioritized)
- Extensive Baseline Learner architecture (not in MVP2)
- Falco/KubeArmor integration details (future/maybe)
- TimescaleDB migration (using PostgreSQL for now)
- Detailed Redis caching (optional, not implemented)
- Attack path algorithms (mentioned but not detailed - needs implementation)

**Reduced**:
- Agent architecture: Focused on what's done (inventory), brief mention of eBPF future
- Storage: Focused on PostgreSQL+AGE, brief NATS mention
- Monitoring: Mentioned but not detailed (Prometheus metrics exist)

### 7. Documentation Improvements

**Better Organization**:
- Clear component boundaries (Agent, Core, Policy, Risk, Graph, CVE, Dashboard)
- Consistent status markers (✅ Complete, ⏳ Partial/In Progress, 📋 Planned)
- Code examples for actual implementations (YAML policies, Cypher queries)
- Data flow diagrams for understanding
- Performance benchmarks from actual testing

**Clarity**:
- Separated "complete" from "in progress" from "planned"
- Removed vague "to be implemented" without status
- Added version info (2.0, MVP2)
- Linked to implementation guides

### 8. Accuracy Fixes

**Technical Corrections**:
- Worker count: 5 per type (20 total) - confirmed accurate
- Resource limits: <100MB agent, 512Mi-2Gi core - confirmed accurate
- Test coverage: 87.5% (28/32 passing) - confirmed accurate
- Database migrations: 14 files - confirmed accurate
- Built-in rules: 25+ CIS rules - confirmed accurate
- Dashboard pages: 9 pages - confirmed accurate

**False Claims Removed**:
- "TimescaleDB for time-series" (not implemented)
- "Redis required" (optional, not in base deployment)
- "Baseline learning active" (concept only, not implemented)
- "eBPF integration in progress" (not started)
- "Attack path algorithms working" (graph engine ready, algorithms not implemented)

## Impact

### For Developers

**Before**: "Is this actually implemented? The doc says 'to be implemented' but I see code..."
**After**: Clear status on every component, matches codebase reality

### For Users

**Before**: "Can I use policy enforcement?" → Unclear, doc says 20% complete
**After**: "Yes, it's ✅ Complete with CEL evaluator and admission webhook"

### For Stakeholders

**Before**: MVP-1 is 80% done (misleading)
**After**: MVP-1 is ✅ Released v4.3.0, MVP-2 is 60% done with specific features listed

## Statistics

### Size
- Old: 3,205 lines
- New: 980 lines
- Reduction: 69.4% (removed 2,225 lines of outdated/future content)

### Status Markers
- ✅ Complete: 52 instances (vs. ~15 in old doc)
- ⏳ In Progress: 18 instances (vs. ~8 in old doc)
- 📋 Planned: 6 instances (vs. unclear in old doc)

### Content Distribution
- 30% Core Components (Agent, Core, Dashboard)
- 25% Policy & Risk Engines
- 15% Storage & Graph
- 10% Data Flow
- 10% Deployment & Getting Started
- 10% Roadmap, Tech Stack, Performance

## Validation Checklist

- [x] All status markers match codebase
- [x] Technology stack is accurate
- [x] MVP milestones reflect reality (MVP1 done, MVP2 60%)
- [x] No "to be implemented" for completed features
- [x] Cross-references to implementation guides
- [x] Diagrams match current architecture
- [x] Performance numbers from actual tests
- [x] Security claims are accurate (mTLS, JWT, etc.)

## Next Steps

1. ✅ Archive old version
2. ✅ Write new version
3. ⏳ Update README.md to link to new architecture
4. ⏳ Review with team for accuracy
5. ⏳ Create supplementary docs:
   - Policy Engine Deep Dive
   - Graph Query Examples
   - CVE/SBOM Roadmap
   - Attack Path Implementation Plan

## Migration Notes

**If you have links to old sections**:

| Old Section | New Section |
|-------------|-------------|
| `## Components / 2.2 Message Queue (NATS JetStream)` | `## Core Components / 2.2 Message Queue (NATS JetStream)` |
| `## Risk Engine (scattered)` | `## Core Components / 4. Risk Engine` |
| Policy Engine (didn't exist) | `## Core Components / 3. Policy Engine` |
| eBPF detailed architecture | `## Core Components / 1. KSAM Agent / Future: eBPF Runtime Monitoring` |
| Attack Path Simulation (long section) | Mentioned briefly in Graph Engine, needs implementation |

**Old version available at**:
`docs/archive/old-architecture/ARCHITECTURE_v1_backup_20251215.md` (3,205 lines)

---

**Changelog Author**: Claude Code
**Review Status**: Pending team review
**Approved By**: TBD
