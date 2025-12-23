# Architecture Documentation

Comprehensive documentation of Fortuna's architecture, design decisions, and system components.

## 📚 Core Documents

### Primary Architecture
- **[README.md](./README.md)** - Complete architecture overview (17KB)
- **[CORE_ONLY_ANALYSIS.md](./CORE_ONLY_ANALYSIS.md)** - Core-Only architecture analysis
- **[KSAM_ADR_FULL.md](./KSAM_ADR_FULL.md)** - Architecture Decision Records

### Historical
- **[ARCHITECTURE_OLD.md](./ARCHITECTURE_OLD.md)** - Previous architecture (for reference)

### Index
- **[INDEX.md](./INDEX.md)** - Architecture document index

### Changelog
- **[changelog/](./changelog/)** - Architecture evolution history

---

## 🎯 Current Architecture: Core-Only

**Important**: Fortuna currently uses a **Core-Only Architecture**.

```
┌─────────────────────────────────────────────────────────┐
│                Kubernetes Cluster                        │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  Fortuna Core (Deployment)                              │
│       ├─→ K8s API Client (collect resources)           │
│       ├─→ SBOM Worker (extract packages)                │
│       ├─→ CVE Matcher (match vulnerabilities)          │
│       ├─→ Risk Engine (calculate scores)                │
│       ├─→ Insight Manager (create findings)             │
│       └─→ REST API (serve dashboard)                    │
│                                                          │
│  PostgreSQL + NATS JetStream                            │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

**Key Points**:
- ✅ **Core handles all processing**: Collection, SBOM, CVE, Insights
- ❌ **Agent is disabled**: Not needed for single-cluster deployments
- ✅ **Simpler deployment**: Single pod vs N pods (agent per node)
- ✅ **Feature complete**: All capabilities available

---

**Next**: [Components](../03-components/)
