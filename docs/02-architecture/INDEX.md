# Architecture Documentation - Index

## 📚 Overview

This directory contains the architectural documentation for Fortuna K8s Management Platform.

---

## 📖 Documents

### Core Architecture
- **[README.md](./README.md)** - Architecture overview and system design
- **[ARCHITECTURE.md](./ARCHITECTURE.md)** - Architecture decision framework
- **[COMPONENTS.md](./COMPONENTS.md)** - Component specifications
- **[Database Schema](./database/SCHEMA_ANALYSIS.md)** - PostgreSQL + Apache AGE schema
- **[REPOSITORY_STRUCTURE.md](./REPOSITORY_STRUCTURE.md)** - Codebase organization

### Design Decisions
- **[ADR-0010: Agent as Data Plane](./ADR-0010-AGENT-AS-DATA-PLANE-MANDATORY.md)** - Agent architecture decision
- **[ADR-0011: Naming Migration](./ADR-0011-NAMING-MIGRATION.md)** - KSAM → Fortuna naming decision
- **[ADR Index](../adr/)** - All architecture decision records

### API Architecture
- **[API Route Classification](./API_ROUTE_CLASSIFICATION_ABC.md)** - API route organization (ABC classification)
- **[API Route Remediation Plan](./API_ROUTE_REMEDIATION_PLAN.md)** - API standardization roadmap
- **[API Architect & Route Standard](./API-ARCHITECT_AND_ROUTE_STANDARD.md)** - API design standards

### Security
- **[Security Guide](../01-getting-started/SECURITY_GUIDE.md)** - Comprehensive security setup (mTLS, RBAC, secrets)

### UI/UX Design
- **[UI Layout Patterns](./FORTUNA_UI_LAYOUT_PATTERNS.md)** - Dashboard UI patterns
- **[UI Design System](./FORTUNA_UI_DESIGN_SYSTEM_03132026.md)** - Design system specification
- **[Component Library](./FORTUNA_COMPONENT_LIBRARY.md)** - Reusable UI components
- **[Data Visualization Guidelines](./FORTUNA_DATA_VISUALIZATION_GUIDELINES.md)** - Chart and visualization standards

---

## 🏗️ High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                 Kubernetes Cluster                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐        ┌──────────────┐                  │
│  │ Fortuna Agent│───────▶│ Fortuna Core │                  │
│  │  (DaemonSet) │ gRPC   │ (Deployment) │                  │
│  └──────────────┘ mTLS   └──────┬───────┘                  │
│                                  │                           │
│                         ┌────────┴────────┐                 │
│                         │                 │                 │
│                  ┌──────▼─────┐    ┌─────▼─────┐           │
│                  │ PostgreSQL │    │   NATS    │           │
│                  │  + AGE     │    │JetStream  │           │
│                  └────────────┘    └───────────┘           │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## 🎯 Key Concepts

### Zero-Trust Architecture
- All communication secured with mTLS
- No service accounts with cluster-admin
- Minimal RBAC permissions

### Event-Driven Design
- NATS JetStream for async processing
- Decoupled components
- Scalable workers

### Graph-Based Analysis
- Apache AGE for attack path modeling
- Relationship tracking
- Attack surface visualization

---

## 📊 Data Flows

### 1. Resource Collection Flow
```
Agent → gRPC → Core → NATS (raw.pods) → Normalizer → NATS (norm.pods)
```

### 2. SBOM Generation Flow
```
NATS (norm.pods) → SBOM Worker → Extract SBOM → PostgreSQL
                                               → NATS (sbom.created)
```

### 3. CVE Matching Flow
```
NATS (sbom.created) → CVE Matcher → Match CVEs → PostgreSQL (cve_matches)
                                               → Create Insights
```

### 4. Risk Scoring Flow
```
Insights → Risk Engine → Calculate Score → PostgreSQL (risk_scores)
```

---

## 🔧 Components

| Component | Type | Purpose |
|-----------|------|---------|
| **Agent** | DaemonSet | Collect K8s resources |
| **Core** | Deployment | Central controller |
| **PostgreSQL** | StatefulSet | Primary data store |
| **NATS** | StatefulSet | Event bus |
| **SBOM Worker** | Core process | Generate SBOMs |
| **CVE Matcher** | Core process | Match vulnerabilities |
| **Risk Engine** | Core process | Calculate risk scores |
| **Policy Engine** | Core process | Evaluate policies |

---

## 📈 Scalability

### Horizontal Scaling
- **Agent**: Scales with nodes (DaemonSet)
- **Core**: Can scale to multiple replicas
- **NATS**: 3-node cluster (raft consensus)
- **PostgreSQL**: Single master (can add read replicas)

### Performance Targets
- SBOM generation: <2 seconds
- CVE matching: <1 second
- Risk scoring: <100ms
- API response: <50ms (p99)

---

## 🔒 Security Considerations

### Authentication
- mTLS for Agent ↔ Core
- API tokens for external access
- ServiceAccount tokens for K8s API

### Authorization
- RBAC for K8s resources
- Policy Engine for admission control
- Row-level security in PostgreSQL (future)

### Data Protection
- Secrets encrypted at rest
- TLS for all network traffic
- No plaintext credentials

---

## 🗺️ Related Documentation

- **[Getting Started](../01-getting-started/README.md)** - Deploy Fortuna
- **[Components](../03-components/)** - Component details
- **[Operations](../05-operations/)** - Run in production
- **[Reference](../06-reference/)** - Technical reference

---

*Last Updated: April 2026 (v1.0.0)*

