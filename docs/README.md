# Fortuna K8s Management Platform - Documentation

**Welcome to Fortuna!** 🚀

Fortuna is a comprehensive Kubernetes security and management platform that provides:
- 🔒 **Security Insights**: RBAC analysis, vulnerability detection, risk scoring
- 📦 **SBOM Management**: Software Bill of Materials generation and tracking
- 🛡️ **CVE Scanning**: Custom vulnerability matching (74,561+ CVEs)
- 📊 **Risk Engine**: Automated risk assessment and prioritization
- 🎯 **Policy Engine**: Admission control and compliance enforcement

---

## 🚀 Quick Start

**New to Fortuna?** Start here:
1. **[START_HERE.md](./START_HERE.md)** - 5-minute overview
2. **[Getting Started Guide](./getting-started/README.md)** - Installation & setup
3. **[Quick Start Tutorial](./getting-started/QUICKSTART.md)** - Your first deployment

---

## 📚 Documentation Structure

### 🎯 [Getting Started](./getting-started/)
- **Installation** - Deploy Fortuna in your cluster
- **Quick Start** - Basic usage examples
- **Configuration** - Environment variables & settings
- **Troubleshooting** - Common issues & solutions

### 🏗️ [Architecture](./architecture/)
- **Overview** - System design & data flows
- **Components** - Core, Agent, and services
- **Database Schema** - PostgreSQL + Apache AGE
- **Event System** - NATS JetStream architecture
- **Security** - mTLS, RBAC, admission control

### 🔧 [Components](./components/)
- **[Agent](./components/agent/)** - DaemonSet resource collector
- **[Core](./components/core/)** - Central controller & API
- **[SBOM Generator](./components/sbom/)** - Custom SBOM extraction
- **[CVE Scanner](./components/cve-scanner/)** - Vulnerability matching
- **[Policy Engine](./components/policy-engine/)** - CEL-based policies
- **[Risk Engine](./components/risk-engine/)** - Risk scoring & insights
- **[Dashboard](./components/dashboard/)** - Web UI (optional)

### 💻 [Development](./development/)
- **Contributing** - How to contribute
- **Development Setup** - Local development environment
- **Testing** - E2E tests, unit tests
- **Performance** - Benchmarks & optimization
- **API Reference** - REST API documentation

### 🚀 [Operations](./operations/)
- **Deployment** - Production deployment guide
- **Monitoring** - Observability & metrics
- **Backup & Restore** - Data management
- **Scaling** - Horizontal scaling strategies
- **Upgrades** - Version upgrade procedures

### 📦 [Migration](./migration/)
- **From KSAM** - Migrating from KSAM to Fortuna
- **Execution Report** - Detailed migration log
- **Rollback Guide** - How to rollback if needed

---

## 📖 Key Documents

### Must-Read
- **[START_HERE.md](./START_HERE.md)** - Start your Fortuna journey
- **[ARCHITECTURE.md](./architecture/README.md)** - Understand the platform
- **[SECURITY.md](./SECURITY.md)** - Security policies & best practices

### Reference
- **[API_REFERENCE.md](./development/API_REFERENCE.md)** - REST API endpoints
- **[DATABASE_SCHEMA.md](./architecture/DATABASE_SCHEMA.md)** - Database structure
- **[CONFIGURATION.md](./getting-started/CONFIGURATION.md)** - All config options

### Guides
- **[QUICKSTART.md](./getting-started/QUICKSTART.md)** - Get started in 10 minutes
- **[TROUBLESHOOTING.md](./getting-started/TROUBLESHOOTING.md)** - Fix common issues
- **[CONTRIBUTING.md](./development/CONTRIBUTING.md)** - Join the project

---

## 🎯 Use Cases

### Security Teams
- **Vulnerability Management**: Track CVEs across all workloads
- **RBAC Analysis**: Detect overprivileged ServiceAccounts
- **Risk Assessment**: Prioritize security issues by risk score
- **Compliance**: Enforce policies via admission control

### DevOps Teams
- **SBOM Tracking**: Know what's running in production
- **Image Security**: Scan container images for vulnerabilities
- **Drift Detection**: Detect unauthorized changes
- **Automation**: Policy-as-code with CEL expressions

### Platform Teams
- **Multi-Tenancy**: Namespace-level isolation
- **Resource Management**: Track and optimize resource usage
- **Attack Path Analysis**: Graph-based threat modeling
- **Reporting**: Generate security reports for audits

---

## 🏗️ Architecture Highlights

```
┌─────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                       │
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

**Key Features**:
- 🔐 **Zero-Trust**: mTLS for all communication
- 📊 **Event-Driven**: NATS JetStream for async processing
- 🗄️ **Graph Database**: Apache AGE for attack path analysis
- 🚀 **Cloud-Native**: Kubernetes-native design
- 🔧 **Extensible**: Plugin-based architecture

---

## 📊 Performance

- **CVE Database**: 74,561 CVEs loaded and indexed
- **SBOM Generation**: ~2-5 seconds per image
- **CVE Matching**: <1 second per SBOM
- **Risk Scoring**: <100ms per resource
- **API Response**: <50ms (p99)

**Scalability**:
- Tested with 1000+ pods
- 10,000+ insights managed
- Horizontal scaling supported

---

## 🛠️ Tech Stack

| Component | Technology |
|-----------|-----------|
| **Language** | Go 1.23+ |
| **Database** | PostgreSQL 15+ |
| **Graph DB** | Apache AGE |
| **Message Bus** | NATS JetStream |
| **API** | Gin (REST) |
| **Policy** | CEL (Common Expression Language) |
| **Container** | Docker, Kubernetes |

---

## 🤝 Community & Support

### Get Help
- **Documentation**: You're reading it!
- **GitHub Issues**: Report bugs or request features
- **Discussions**: Ask questions and share ideas

### Contributing
We welcome contributions! See [CONTRIBUTING.md](./development/CONTRIBUTING.md) for:
- Code contributions
- Documentation improvements
- Bug reports
- Feature requests

---

## 📝 Recent Updates

### v2.0.0 - Fortuna (December 2024)
- ✅ Renamed from KSAM to Fortuna
- ✅ Database optimization (96% insights reduction)
- ✅ Custom SBOM generator (zero external tools)
- ✅ Enhanced CVE matching engine
- ✅ Improved risk scoring algorithm (V2)
- ✅ Better documentation structure

See [CHANGELOG.md](./CHANGELOG.md) for full history.

---

## 🗺️ Roadmap

### Q1 2025
- [ ] Web Dashboard improvements
- [ ] Multi-cluster support
- [ ] Enhanced reporting
- [ ] CIS Kubernetes Benchmark integration

### Q2 2025
- [ ] Machine learning for risk prediction
- [ ] Advanced attack path visualization
- [ ] SLSA provenance verification
- [ ] Supply chain security features

See [ROADMAP.md](./ROADMAP.md) for details.

---

## 📄 License

This project is licensed under the MIT License - see [LICENSE](../LICENSE) for details.

---

## 🙏 Acknowledgments

Built with ❤️ using amazing open-source projects:
- Kubernetes
- PostgreSQL + Apache AGE
- NATS
- Gin Web Framework
- And many more...

---

**Ready to secure your Kubernetes cluster?**

👉 **[Start Here](./START_HERE.md)** | 📚 **[Quick Start](./getting-started/QUICKSTART.md)** | 🔒 **[Security Guide](./SECURITY.md)**

---

*Fortuna K8s Management Platform - Secure, Scalable, Simple* 🚀
