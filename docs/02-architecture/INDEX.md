# Architecture Documentation - Index

## 📚 Overview

This directory contains the architectural documentation for Fortuna K8s Management Platform.

---

## 📖 Documents

### Core Architecture
- **[README.md](./README.md)** - Architecture overview and system design
- **[DATA_FLOWS.md](./DATA_FLOWS.md)** - Data flow diagrams and explanations
- **[Database Schema](./database/SCHEMA_ANALYSIS.md)** - PostgreSQL + Apache AGE schema
- **[EVENT_SYSTEM.md](./EVENT_SYSTEM.md)** - NATS JetStream architecture

### Design Decisions
- **[ADR (Architecture Decision Records)](./KSAM_ADR_FULL.md)** - Key architectural decisions
- **[WHY_FORTUNA.md](./WHY_FORTUNA.md)** - Why we built Fortuna
- **[DESIGN_PRINCIPLES.md](./DESIGN_PRINCIPLES.md)** - Core design principles

### Security
- **[SECURITY_MODEL.md](./SECURITY_MODEL.md)** - Security architecture
- **[MTLS_SETUP.md](./MTLS_SETUP.md)** - mTLS configuration
- **[RBAC_DESIGN.md](./RBAC_DESIGN.md)** - RBAC implementation

### Components
- **[COMPONENT_DIAGRAM.md](./COMPONENT_DIAGRAM.md)** - Component relationships
- **[DEPLOYMENT_TOPOLOGY.md](./DEPLOYMENT_TOPOLOGY.md)** - Deployment patterns

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

- **[Getting Started](../getting-started/README.md)** - Deploy Fortuna
- **[Components](../components/README.md)** - Component details
- **[Operations](../operations/README.md)** - Run in production

---

*Last Updated: December 2024 (v2.0 - Fortuna)*

