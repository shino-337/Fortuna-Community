# Fortuna Platform Components

**Version**: 2.0  
**Last Updated**: 2026-01-29

---

## Overview

Fortuna consists of three main components: **Agent**, **Core**, and **Dashboard**, along with supporting infrastructure components.

---

## Core Components

### 1. Agent (DaemonSet)

**Purpose**: Node-level security monitoring and SBOM extraction

**Deployment**: Kubernetes DaemonSet (runs on all nodes)

**Responsibilities**:
- Pod detection and monitoring via local pod watcher
- SBOM extraction from container images
- Runtime event collection
- gRPC communication with Core (mTLS)

**Key Features**:
- **Asynchronous Work Queue**: Prevents blocking during SBOM extraction
- **Multi-Parser Support**: dpkg, apk, rpm, npm, pip, gomod
- **OS-Aware Parsing**: Only runs relevant parsers per OS
- **Containerd Integration**: Direct containerd socket access
- **Runtime Events**: Collects and forwards runtime security signals

**Configuration**:
- `CORE_GRPC_ENDPOINT`: Core gRPC endpoint
- `CORE_HTTP_ENDPOINT`: Core HTTP endpoint (optional)
- `CLUSTER_ID`: Cluster identifier
- `SYNC_INTERVAL`: Sync interval (default: 5m)
- `TLS_ENABLED`: Enable mTLS (default: true)
- `CONTAINERD_SOCKET`: Containerd socket path
- `CONTAINERD_NAMESPACE`: Containerd namespace (default: k8s.io)

**Resources**:
- Requests: 100m CPU, 256Mi memory
- Limits: 500m CPU, 1Gi memory

---

### 2. Core (Deployment)

**Purpose**: Central processing, storage, and API services

**Deployment**: Kubernetes Deployment (runs on control-plane nodes)

**Responsibilities**:
- SBOM storage and management
- CVE matching and vulnerability detection
- Security insights generation
- Policy evaluation and enforcement
- Pod Capability Engine (PCE)
- Attack path analysis
- Runtime signal processing
- REST API (51+ endpoints)
- gRPC API for Agent communication
- Admission webhook (optional)

**Key Features**:
- **NATS JetStream**: Event-driven architecture
- **PostgreSQL**: Persistent storage with automatic migrations
- **PCE Scheduler**: Periodic capability evaluation (default: 6h)
- **Workers**: Asynchronous processing (CVE matcher, correlator, risk)
- **Admission Webhook**: Policy enforcement at admission time
- **Metrics**: Prometheus metrics endpoint (`/metrics`)

**Configuration**:
- `DATABASE_URL`: PostgreSQL connection string
- `NATS_ENDPOINT`: NATS server endpoint (default: `nats://nats-client.fortuna.svc.cluster.local:4222`)
- `HTTP_PORT`: HTTP server port (default: 8080)
- `GRPC_PORT`: gRPC server port (default: 9090)
- `TLS_ENABLED`: Enable mTLS (default: true)
- `AUTH_ENABLED`: Enable JWT authentication (default: true)
- `PCE_SCHEDULER_ENABLED`: Enable PCE scheduler (default: true)
- `PCE_SCHEDULER_INTERVAL`: Scheduler interval (default: 6h)
- `WEBHOOK_TLS_CERT_PATH`: Webhook TLS certificate path
- `WEBHOOK_TLS_KEY_PATH`: Webhook TLS key path

**Resources**:
- Requests: 100m CPU, 256Mi memory
- Limits: 1000m CPU, 1Gi memory

**API Endpoints**:
- REST API: `/api/v1/*` (51+ endpoints)
- gRPC API: Port 9090
- Metrics: `/metrics` (Prometheus format)
- Health: `/healthz`, `/ready`

---

### 3. Dashboard (Deployment)

**Purpose**: Web-based user interface

**Deployment**: Kubernetes Deployment (runs on control-plane nodes)

**Responsibilities**:
- Security dashboard and visualization
- Risk center and insights management
- SBOM analysis and vulnerability browsing
- Attack path visualization
- Pod capabilities and runtime signals monitoring
- Capability metadata browser

**Key Features**:
- **React + TypeScript**: Modern frontend framework
- **Vite**: Fast build tool
- **Real-time Updates**: API-driven data refresh
- **Multiple Views**: Dashboard, Risks, SBOM, Attack Paths, Capabilities, Clusters

**Configuration**:
- `VITE_CORE_API_URL`: Core API URL (optional, defaults to `/api/v1`)

**Resources**:
- Requests: 50m CPU, 64Mi memory
- Limits: 100m CPU, 128Mi memory

---

## Infrastructure Components

### PostgreSQL

**Purpose**: Primary database for all Fortuna data

**Storage**:
- SBOMs and components
- CVEs and vulnerability matches
- Security insights
- Pod capabilities and metadata
- Attack steps
- Runtime signals
- Promotion rules
- Policies and rules

**Features**:
- Automatic migrations (36+ migrations)
- JSONB support for flexible data
- Unique constraints for deduplication
- Indexes for performance

### NATS JetStream

**Purpose**: Message queue for event-driven processing

**Configuration**:
- 3-replica cluster (production)
- JetStream enabled
- Multiple streams for different event types

**Streams**:
- `fortuna-raw`: Raw events from agents
- `fortuna-events`: Normalized events and SBOM/CVE processing
- `fortuna-insights`: Insight generation events
- `fortuna-normalized`: Normalized event processing

---

## Pod Capability Engine (PCE)

### Overview

PCE is a core feature that detects, tracks, and analyzes security capabilities of Kubernetes pods.

### Capability Types

**Static Capabilities** (from PodSpec):
- `ESC_PRIV_POD`: Privileged containers
- `ESC_HOSTPID_POD`: Host PID namespace access
- `ESC_HOSTIPC_POD`: Host IPC namespace access
- `ESC_HOSTPATH_NODE`: HostPath mounts
- `NET_HOSTNETWORK`: Host network access
- `ID_TOKEN_POD`: ServiceAccount token access
- `API_RBAC_WRITE_CLUSTER`: RBAC write permissions
- `CTRL_CONTROL_PLANE_POD`: Control plane namespace

**Runtime Capabilities** (from runtime signals):
- `ESC_RUNTIME_PROC_ROOT`: Container escape via /proc/1/root
- `ESC_RUNTIME_ACTIVE`: Active escape confirmed
- `ESC_RUNTIME_PROBE`: Escape potential detected

### Capability States

1. **detected**: Initial state from static analysis
2. **confirmed**: Promoted when runtime signals match rules
3. **exploited**: Active exploitation detected
4. **chained**: Multiple capabilities combined

### Components

- **CapabilityStateController (CSC)**: Single owner for state transitions
- **SignalAdapter**: Converts runtime events to semantic signals
- **AttackStepInference**: Generates attack steps from exploited capabilities
- **PromotionRules**: Rules for state promotion based on signals

---

## Data Flow

### SBOM Flow

```
Pod Created
  ↓
Agent Detects Pod
  ↓
Agent Extracts SBOM
  ↓
Agent → Core (gRPC)
  ↓
Core Stores SBOM
  ↓
Core Publishes Event (NATS)
  ↓
CVE Matcher Worker
  ↓
CVE Matching
  ↓
Insight Generation
```

### PCE Flow

```
Pod Sync
  ↓
PCE Evaluation (Static)
  ↓
Capability Detection
  ↓
Runtime Events
  ↓
Signal Adaptation
  ↓
State Promotion (CSC)
  ↓
Attack Step Inference
  ↓
Attack Path Analysis
```

---

## Security

### mTLS

All inter-component communication uses mTLS:
- Agent ↔ Core (gRPC)
- Core ↔ Core (internal, if multiple replicas)

### Authentication

- JWT-based authentication for REST API
- Configurable via `AUTH_ENABLED` environment variable

### RBAC

- ServiceAccounts with minimal required permissions
- ClusterRoles and ClusterRoleBindings for Agent
- Namespace-scoped roles for Core

---

## Monitoring

### Metrics

Core service exposes Prometheus metrics at `/metrics`:
- Database connection pool metrics
- Worker performance metrics
- CVE matching metrics
- API request metrics

### Health Checks

- **Liveness**: `/healthz` endpoint
- **Readiness**: `/ready` endpoint

---

## Deployment

### Kubernetes Resources

1. **Agent**: DaemonSet (all nodes)
2. **Core**: Deployment (control-plane nodes)
3. **Dashboard**: Deployment (control-plane nodes)
4. **PostgreSQL**: StatefulSet (optional, can use external)
5. **NATS**: StatefulSet (optional, can use external)

### Helm Charts

- `helm/fortuna/`: Recommended production chart
- `helm/ksam/`: Legacy chart (backward compatibility)

See [Deployment Guide](PRODUCTION_DEPLOYMENT.md) for detailed instructions.

---

**Last Updated**: 2026-01-29
