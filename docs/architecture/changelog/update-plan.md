# ARCHITECTURE.md Update Plan

**Date**: 2025-12-15
**Current Version**: 3,205 lines (outdated since MVP1)
**Target**: Updated to reflect MVP2 current state

## Issues Found

### 1. Outdated Implementation Status

| Component | Doc Says | Reality | Fix Needed |
|-----------|----------|---------|------------|
| NATS JetStream | 🔧 To be implemented | ✅ Complete | Update status |
| Policy Engine | ⚠️ 20% Complete | ✅ Complete | Update status |
| Admission Webhook | Not mentioned | ✅ Complete | Add section |
| Apache AGE | ⏳ To be implemented | ✅ Complete | Update status |
| mTLS | ⏳ To be implemented | ✅ Complete | Update status |
| CVE Scanning | Not mentioned | ⏳ Partial | Add section |
| SBOM | Not mentioned | ⏳ Partial | Add section |
| eBPF Runtime | Mentioned as priority | ⏳ Not started | Update priority |

### 2. Missing MVP2 Features

Features implemented but not documented:
- **Policy Engine** (complete with CEL evaluator, enforcement, remediation)
- **Admission Webhook** (ValidatingWebhookConfiguration)
- **Risk Scoring V2** (new algorithm with auto-resolution)
- **Insights Lifecycle** (soft delete, auto-resolved status)
- **Dashboard D3 Visualization** (force-directed graph with pink branding)
- **CVE Integration** (partial - schema ready, Trivy integration incomplete)
- **SBOM Generation** (partial - extractors implemented)

### 3. Incorrect Roadmap

| Roadmap Item | Doc Says | Reality |
|--------------|----------|---------|
| MVP-1 | 80% Complete | 100% Complete (released v4.3.0) |
| MVP-2 | 0% | ~60% Complete (policy engine, webhook, risk v2 done) |
| Q1 Foundation | In progress | Complete |
| Q2 Observability | 0% | Apache AGE done, eBPF not started |

### 4. Technology Stack Updates

| Technology | Doc Says | Reality |
|------------|----------|---------|
| State Management (Frontend) | Redux Toolkit | Zustand |
| Graph | Not specified | Apache AGE (PostgreSQL extension) |
| Build Tool | Vite | ✅ Correct |
| Policy Engine | Not specified | Google CEL |

### 5. Incorrect Differentiation

Doc claims differentiators not yet implemented:
- Attack path analysis (UI ready, backend partial - should clarify)
- Blast radius (graph engine ready, needs algorithm)
- Policy automation (✅ complete - not "to be implemented")

## Update Strategy

### Phase 1: Status Updates (Quick Wins)
- Update all ✅ ⏳ ⚠️ 🔧 markers to reflect reality
- Fix MVP milestone percentages
- Update technology stack

### Phase 2: Add Missing Sections
- Policy Engine detailed architecture
- Admission Webhook integration
- CVE/SBOM current state
- Risk Scoring V2 algorithm

### Phase 3: Remove Outdated Content
- Remove "to be implemented" for completed features
- Archive unrealistic timelines
- Remove duplicate/conflicting information

### Phase 4: Restructure
- Move implementation details to separate docs
- Keep ARCHITECTURE.md high-level
- Link to implementation guides

## Proposed New Structure

```markdown
# KSAM Architecture

## Overview [KEEP]
- Update to reflect MVP2 state
- Add version info (MVP1 released, MVP2 in progress)

## System Architecture [UPDATE]
- Update diagram to include:
  - Policy Engine
  - Admission Webhook
  - Apache AGE
  - CVE/SBOM (partial)

## Core Components [MAJOR UPDATE]

### 1. KSAM Agent
- ✅ Inventory collection (COMPLETE)
- ⏳ eBPF runtime (NOT STARTED - deprioritized)
- Update status markers

### 2. KSAM Core Controller
- ✅ Ingest API (COMPLETE)
- ✅ NATS JetStream (COMPLETE)
- ✅ Worker Pool (COMPLETE)
- ✅ Policy Engine (COMPLETE) **NEW**
- ✅ Admission Webhook (COMPLETE) **NEW**
- ⏳ CVE Scanning (PARTIAL) **NEW**
- ⏳ SBOM Generation (PARTIAL) **NEW**

### 3. KSAM Dashboard
- ✅ All pages implemented
- ✅ D3 graph visualization
- Update technology stack (Zustand not Redux)

### 4. Storage Layer [UPDATE]
- ✅ PostgreSQL with Apache AGE (COMPLETE)
- ✅ NATS JetStream (COMPLETE)
- ⏳ Redis (OPTIONAL - may not implement)
- ⏳ TimescaleDB (DEPRIORITIZED)

## Data Flow [KEEP - Minor updates]

## Policy Engine [NEW SECTION]
- CEL-based evaluation
- YAML rule format
- Enforcement modes (Block/Warn/Audit)
- Remediation guidance
- Admission webhook integration

## Risk Engine [UPDATE]
- V2 algorithm with auto-resolution
- Insight lifecycle management
- Soft delete with 30-day retention
- Risk scoring formula

## Graph Engine [UPDATE]
- ✅ Apache AGE implementation (COMPLETE)
- Query service API
- Performance characteristics
- Attack path analysis (partial)

## CVE & SBOM Integration [NEW SECTION]
- Current state (partial implementation)
- Schema and data models
- Integration points (planned)
- Trivy integration (incomplete)

## Security [UPDATE]
- ✅ mTLS (COMPLETE)
- ✅ JWT Authentication (COMPLETE)
- ✅ RBAC (COMPLETE)
- Add admission webhook security

## Deployment [UPDATE]
- Remove outdated Helm references
- Add current deployment method
- Update resource requirements

## Development Roadmap [COMPLETE REWRITE]
### MVP1 - Foundation ✅ COMPLETE
- Released v4.3.0
- All core features implemented

### MVP2 - Enterprise Features ⏳ 60% COMPLETE
- ✅ Policy engine with CEL
- ✅ Admission webhook
- ✅ Risk Scoring V2
- ✅ Apache AGE graph engine
- ⏳ CVE scanning (partial)
- ⏳ SBOM generation (partial)
- ⏳ eBPF runtime (not started)

### MVP3 - Advanced Analytics [PLANNING]
- Attack path algorithms
- Blast radius calculation
- ML-based anomaly detection
- Cross-cluster correlation

## Technology Stack [UPDATE]
- Fix frontend state management (Zustand)
- Add Google CEL
- Add Apache AGE
- Clarify current vs planned

## Performance & Scalability [KEEP]

## Testing [UPDATE]
- Current test coverage: 87.5%
- Link to test documentation

## Getting Started [UPDATE]
- Link to setup guides
- Remove outdated commands
- Add current deployment process

```

## Changes Summary

| Section | Action | Priority |
|---------|--------|----------|
| Overview | Update | High |
| System Diagram | Redraw | High |
| Component Status | Update all | High |
| Policy Engine | Add new section | High |
| Admission Webhook | Add new section | High |
| Graph Engine | Update status | High |
| CVE/SBOM | Add new section | Medium |
| Roadmap | Complete rewrite | High |
| Technology Stack | Update | Medium |
| eBPF | Deprioritize | Low |
| Baseline Learner | Remove (not in mvp2) | Low |
| Falco Integration | Move to future | Low |

## Validation Checklist

- [ ] All status markers match codebase
- [ ] Technology stack is accurate
- [ ] MVP milestones reflect reality
- [ ] No mentions of "to be implemented" for completed features
- [ ] Cross-references to implementation guides
- [ ] Diagrams match current architecture
- [ ] Performance numbers are realistic
- [ ] Security claims are accurate

## Next Steps

1. Create updated ARCHITECTURE.md (70% rewrite)
2. Archive old version as ARCHITECTURE_v1.md
3. Update README.md to link to new architecture
4. Create supplementary docs for:
   - Policy Engine Deep Dive
   - Graph Engine Guide
   - CVE/SBOM Roadmap
5. Update architecture review doc
