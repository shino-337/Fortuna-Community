# Components Documentation - Index

## 📚 Overview

This directory contains detailed documentation for each Fortuna component.

---

## 🔧 Core Components

### [Fortuna Core](./core/README.md)
**Central controller and API server**
- API endpoints (REST)
- gRPC server (Agent communication)
- Event processing (NATS)
- Admission webhook
- Database migrations

**Key Features**:
- ✅ HTTP/gRPC dual servers
- ✅ Event-driven architecture
- ✅ Policy evaluation
- ✅ Risk scoring

---

### [Fortuna Agent](./agent/README.md)
**DaemonSet for resource collection**
- Collects Pods, ServiceAccounts, RBAC
- Sends data to Core via gRPC
- Watches for resource changes
- Periodic full sync

**Key Features**:
- ✅ Low resource footprint
- ✅ Secure mTLS communication
- ✅ Automatic reconnection
- ✅ Configurable sync interval

---

## 🛡️ Security Components

### [SBOM Generator](./sbom/README.md)
**Custom Software Bill of Materials extraction**
- Extracts SBOMs from container images
- Supports multiple package managers
- No external tools required
- Cached for performance

**Supported Formats**:
- ✅ Debian/Ubuntu (dpkg)
- ✅ Alpine (apk)
- ✅ RedHat/CentOS (rpm)
- ✅ Python (pip)
- ✅ Node.js (npm)
- ✅ Java (Maven/Gradle)
- ✅ Go modules

📖 **[Read More](./sbom/README.md)**

---

### [CVE Scanner](./cve-scanner/README.md)
**Vulnerability detection and matching**
- Matches SBOMs against CVE database
- 74,561+ CVEs from OSV.dev
- PostgreSQL-based (no external API calls)
- Version range matching

**Features**:
- ✅ Fast matching (<1 second per SBOM)
- ✅ Accurate version comparison
- ✅ CVSS scoring
- ✅ CWE mapping

📖 **[Read More](./cve-scanner/README.md)**

---

### [Policy Engine](./policy-engine/README.md)
**CEL-based policy evaluation**
- Admission webhook integration
- CEL (Common Expression Language) expressions
- Block/Audit/Warn modes
- Custom policy rules

**Example Policy**:
```yaml
apiVersion: policy.fortuna.io/v1
kind: Policy
metadata:
  name: block-privileged
spec:
  match:
    kind: Pod
  deny:
    conditions:
    - expression: "object.spec.containers.all(c, !c.securityContext.privileged)"
      message: "Privileged containers are not allowed"
```

📖 **[Read More](./policy-engine/README.md)**

---

### [Risk Engine](./risk-engine/README.md)
**Risk scoring and insight generation**
- Multi-factor risk calculation
- Weighted scoring algorithm
- Insight lifecycle management
- Deduplication logic

**Risk Factors**:
- 🔴 Vulnerabilities (80% weight)
- 🟡 RBAC issues (10% weight)
- 🟢 Network exposure (10% weight)

**Score Range**: 0-100 (Higher = More Risk)

📖 **[Read More](./risk-engine/README.md)**

---

## 📊 Optional Components

### [Dashboard](./dashboard/README.md) (Coming Soon)
**Web UI for Fortuna**
- Insights visualization
- SBOM browser
- Risk score charts
- Policy management

**Status**: Under development (Q1 2025)

---

## 🔗 Component Communication

```
┌─────────────┐
│   Agent     │ (gRPC/mTLS)
└─────┬───────┘
      │
      ▼
┌─────────────┐
│    Core     │ (API/Webhook)
└─────┬───────┘
      │
      ├───────────────┬──────────────┬──────────────┐
      ▼               ▼              ▼              ▼
┌──────────┐   ┌──────────┐  ┌──────────┐  ┌──────────┐
│   SBOM   │   │   CVE    │  │  Policy  │  │   Risk   │
│  Worker  │   │  Matcher │  │  Engine  │  │  Engine  │
└────┬─────┘   └────┬─────┘  └────┬─────┘  └────┬─────┘
     │              │             │             │
     └──────────────┴─────────────┴─────────────┘
                    │
              ┌─────▼──────┐
              │ PostgreSQL │
              │  + NATS    │
              └────────────┘
```

---

## 📖 Component Documentation

### By Category

**Security**:
- [SBOM Generator](./sbom/README.md)
- [CVE Scanner](./cve-scanner/README.md)
- [Policy Engine](./policy-engine/README.md)
- [Risk Engine](./risk-engine/README.md)

**Infrastructure**:
- [Core Controller](./core/README.md)
- [Agent](./agent/README.md)

**Future**:
- [Dashboard](./dashboard/README.md)
- [Graph Engine](./graph-engine/README.md)

---

## 🔧 Development

### Adding a New Component

1. **Create directory**: `components/my-component/`
2. **Add README.md**: Component documentation
3. **Update this INDEX.md**: Add link and description
4. **Create code**: `KSAM/core/pkg/my-component/`
5. **Add tests**: `KSAM/core/pkg/my-component/*_test.go`
6. **Update architecture**: `architecture/COMPONENT_DIAGRAM.md`

### Component Checklist

- [ ] README.md with overview
- [ ] API documentation
- [ ] Configuration options
- [ ] Testing guide
- [ ] Performance benchmarks
- [ ] Troubleshooting section

---

## 📊 Component Matrix

| Component | Language | Dependencies | Performance |
|-----------|----------|-------------|-------------|
| **Core** | Go 1.23 | PostgreSQL, NATS | <50ms API |
| **Agent** | Go 1.23 | None | <10MB RAM |
| **SBOM** | Go 1.23 | Docker (optional) | 2-5s/image |
| **CVE** | Go 1.23 | PostgreSQL | <1s/SBOM |
| **Policy** | CEL | Core | <10ms/eval |
| **Risk** | Go 1.23 | PostgreSQL | <100ms/score |

---

## 🗺️ Related Documentation

- **[Architecture](../architecture/README.md)** - System design
- **[Development](../development/README.md)** - Developer guide
- **[Operations](../operations/README.md)** - Run in production

---

*Last Updated: December 2024 (v2.0 - Fortuna)*

