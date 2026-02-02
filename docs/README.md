# Fortuna Platform Documentation

**Version**: 2.0  
**Last Updated**: 2026-01-29

---

## Welcome to Fortuna

Fortuna is a comprehensive security and risk management platform for Kubernetes clusters. It provides real-time vulnerability detection, SBOM extraction, CVE matching, and security insights generation.

---

## Quick Start

1. **Prepare Environment**: [Environment Preparation Guide](ENVIRONMENT_PREPARATION.md)
2. **Build Images**: [Build Guide](BUILD_GUIDE.md)
3. **Deploy**: [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md)
4. **Verify**: Check deployment status and run end-to-end tests

---

## Documentation Index

### Getting Started

- [Environment Preparation](ENVIRONMENT_PREPARATION.md) - Prepare your Kubernetes cluster
- [Build Guide](BUILD_GUIDE.md) - Build Core and Agent components
- [Production Deployment Guide](PRODUCTION_DEPLOYMENT.md) - Complete deployment instructions

### Architecture & Design

- [Architecture Documentation](ARCHITECTURE.md) - System architecture and components
- [Components Documentation](COMPONENTS.md) - Detailed component breakdown
- [Database Schema](MIGRATIONS.md#database-schema) - Complete database schema reference
- [Migration Guide](MIGRATIONS.md) - Database migrations and schema management

### Operations

- [API Reference](API_REFERENCE.md) - REST API documentation
- [Troubleshooting](PRODUCTION_DEPLOYMENT.md#troubleshooting) - Common issues and solutions

### Components

- [Core Components Analysis](CORE_COMPONENTS_ANALYSIS.md) - Core component breakdown
- [Agent Components Analysis](AGENT_COMPONENTS_ANALYSIS.md) - Agent component breakdown
- [Agent-Core Flow](AGENT_CORE_FLOW.md) - Data flow between Agent and Core

### Testing

- [Test Results](test-results/) - Latest test execution results

---

## System Components

### Core

Central processing component that handles:
- SBOM storage and management
- CVE matching and vulnerability detection
- Insight generation
- Policy evaluation
- API services

**Deployment**: Kubernetes Deployment (runs on master node)

### Agent

Node-level component that:
- Monitors pods on each node
- Extracts SBOMs from container images
- Communicates with Core via gRPC

**Deployment**: Kubernetes DaemonSet (runs on all nodes)

### Infrastructure

- **PostgreSQL**: Database for SBOMs, CVEs, insights, capabilities, attack steps
- **NATS JetStream**: Message queue for event processing (3-replica cluster)
- **Metrics**: Core service exposes `/metrics` endpoint (Prometheus format)

---

## Architecture Overview

```
┌─────────────┐
│   Agent     │ (DaemonSet - one per node)
│  - Pod Watch│
│  - SBOM Ext │
└──────┬──────┘
       │ gRPC (mTLS)
       ▼
┌─────────────┐
│    Core     │ (Deployment)
│  - Storage  │
│  - CVE Match│
│  - Insights │
└──────┬──────┘
       │
       ├──► PostgreSQL (Database)
       └──► NATS JetStream (Events)
```

---

## Key Features

- ✅ **Automatic SBOM Extraction**: Extracts SBOMs from container images using multiple parsers
- ✅ **Real-time CVE Matching**: Matches CVEs against extracted packages
- ✅ **Security Insights**: Generates actionable security insights
- ✅ **Policy Engine**: Configurable security policies with CEL-based evaluation
- ✅ **Pod Capability Engine (PCE)**: Runtime capability detection and attack step inference
- ✅ **Attack Path Analysis**: Graph-based attack path visualization
- ✅ **Runtime Signals**: Real-time security signal detection and correlation
- ✅ **High Availability**: NATS cluster with 3 replicas
- ✅ **Scalable**: Horizontal scaling support
- ✅ **Secure**: mTLS for all inter-component communication
- ✅ **Observability**: Metrics endpoint (`/metrics`) for Prometheus scraping

---

## Production Readiness

Fortuna is production-ready with:
- ✅ Comprehensive test coverage (80% pass rate)
- ✅ High availability (NATS cluster, multiple replicas)
- ✅ Security (mTLS, RBAC, secure defaults)
- ✅ Observability (Prometheus metrics endpoint)
- ✅ Documentation and operational guides

---

## Support

For issues and questions:
1. Check [Troubleshooting Guide](PRODUCTION_DEPLOYMENT.md#troubleshooting)
2. Review logs: `kubectl logs -n fortuna -l app.kubernetes.io/component=core`
3. Check [Architecture Documentation](ARCHITECTURE.md) for system design

---

## License

[Add your license information here]

---

**Last Updated**: 2026-01-29

---

## Documentation Structure

The documentation is organized into the following structure:

- **Root Level**: Essential guides and main entry points
- **01-getting-started/**: Getting started guides
- **02-architecture/**: Architecture and design documentation
- **03-components/**: Component-specific documentation
- **04-development/**: Development guides
- **05-operations/**: Operations and troubleshooting
- **06-reference/**: Reference materials and historical docs
- **07-guides/**: How-to guides
- **08-tutorials/**: Tutorials
- **test-results/**: Test execution results

For detailed structure information, see [Documentation Structure](DOCUMENTATION_STRUCTURE.md).
