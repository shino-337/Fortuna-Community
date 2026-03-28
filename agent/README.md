# Fortuna Agent

**Version**: 1.0.0  
**Status**: Production Ready

Fortuna Agent is a lightweight DaemonSet of **FortunaK8s** (K8S Security & Risk Management Platform). It runs on each Kubernetes node to detect pods, extract SBOMs (Software Bill of Materials) from container images, and send them to Fortuna Core for security analysis.

---

## Overview

Fortuna Agent operates at the data plane, focusing on pod detection and SBOM extraction. It does **not** perform CVE matching (that's done in Core). The Agent uses an asynchronous work queue to prevent blocking pod detection during slow SBOM extraction operations.

### Key Responsibilities

- **Pod Detection**: Real-time monitoring of pods on the local node
- **SBOM Extraction**: Extract Software Bill of Materials from container images
- **Asynchronous Processing**: Work queue prevents blocking during SBOM extraction
- **gRPC Communication**: Send SBOMs to Core via secure gRPC with mTLS support
- **Multi-Parser Support**: 12 parsers — **dpkg**, **apk**, **rpm**, **npm**, **pip**, **gomod**, **gobinary**, **maven**, **cargo**, **ruby** (Gemfile.lock), **nuget** (packages.lock.json), **distroless**

---

## Architecture

### Components

```
agent/
├── cmd/
│   └── main.go                    # Application entry point
├── internal/
│   ├── watcher/                   # Pod watcher
│   │   └── local_pod_watcher.go  # Local node pod detection
│   ├── sbom/                      # SBOM extraction
│   │   ├── processor.go          # SBOM processing logic
│   │   ├── extractor.go          # Image SBOM extraction
│   │   ├── parser/                # Package parsers
│   │   │   ├── dpkg.go           # Debian dpkg parser
│   │   │   ├── apk.go            # Alpine apk parser
│   │   │   ├── rpm.go            # RPM parser
│   │   │   ├── npm.go            # npm parser
│   │   │   ├── pip.go            # pip parser
│   │   │   ├── gomod.go          # Go modules parser
│   │   │   ├── maven.go          # Maven pom.xml (Tier 3)
│   │   │   ├── cargo.go          # Cargo.lock (Tier 3)
│   │   │   ├── ruby.go           # Gemfile.lock (Tier 3)
│   │   │   └── nuget.go          # packages.lock.json (Tier 3)
│   │   └── queue.go              # Work queue for async processing
│   ├── client/                    # gRPC client
│   │   ├── grpc_client_mtls.go   # mTLS-enabled gRPC client
│   │   └── ...
│   ├── k8s/                       # Kubernetes client
│   │   └── client.go              # K8s client setup
│   └── config/                    # Configuration
│       └── config.go              # Config loading
├── deploy/
│   ├── rbac.yaml                  # RBAC permissions
│   └── daemonset.yaml            # DaemonSet manifest
├── Dockerfile
├── go.mod
└── go.sum
```

---

## Features

### 1. Local Pod Detection

- **Node-Specific**: Only monitors pods on the local node
- **Real-Time**: Uses Kubernetes Informers for immediate pod detection
- **Efficient**: Filters pods by node name to reduce API calls

### 2. SBOM Extraction

- **Multi-Parser Support**:
  - **dpkg**: Debian/Ubuntu packages (`/var/lib/dpkg/status`)
  - **apk**: Alpine packages (`/lib/apk/db/installed`)
  - **rpm**: RedHat/CentOS packages (Fortuna inventory or `rpmdb.sqlite` fallback)
  - **npm**: Node.js packages (`package-lock.json`, `node_modules/*/package.json`)
  - **pip**: Python packages (`requirements.txt`, `*.dist-info/METADATA`)
  - **gomod**: Go modules (`go.sum`, `go.mod`)
  - **gobinary**: Go binaries via `debug/buildinfo`
  - **maven**: Java packages (`pom.xml`)
  - **cargo**: Rust crates (`Cargo.lock`)
  - **ruby**: Ruby gems (`Gemfile.lock`)
  - **nuget**: .NET packages (`packages.lock.json`, `project.assets.json` targets)
  - **distroless**: Control-plane binaries via signature allowlist

- **OS-Aware Parsing**: Only runs relevant parsers based on detected OS
- **PURL Support**: Generates Package URLs (PURL) for all components
- **Metadata Extraction**: OS name, version, architecture

### 3. Asynchronous Work Queue

- **Non-Blocking**: Pod detection continues while SBOM extraction runs
- **Parallel Processing**: Multiple workers process pods concurrently
- **Configurable Workers**: Default 3 workers (configurable)
- **Queue Management**: Prevents memory buildup with bounded queue

### 4. gRPC Communication

- **mTLS Support**: Mutual TLS for secure communication with Core
- **Automatic Reconnection**: Handles network interruptions
- **Heartbeat**: Periodic health checks
- **Error Handling**: Retry logic for transient failures

---

## Configuration

### Environment Variables

**Core Connection**:
- `CORE_GRPC_ENDPOINT`: Core gRPC endpoint (required)
  - Example: `fortuna-core.fortuna.svc.cluster.local:9090`

**Agent Identity**:
- `AGENT_ID`: Unique agent identifier (default: auto-generated)
- `NODE_NAME`: Kubernetes node name (required, auto-detected)
- `NODE_ID`: Node identifier (default: node name)

**TLS/mTLS**:
- `TLS_ENABLED`: Enable mTLS (default: `true`)
- `TLS_CERT_PATH`: Client certificate path (default: `/etc/fortuna/tls/client/tls.crt`)
- `TLS_KEY_PATH`: Client private key path (default: `/etc/fortuna/tls/client/tls.key`)
- `TLS_CA_CERT_PATH`: CA certificate path (default: `/etc/fortuna/tls/client/ca.crt`)

**Containerd**:
- `CONTAINERD_SOCKET`: Containerd socket path (default: `/run/containerd/containerd.sock`)

**Logging**:
- `LOG_LEVEL`: Log level (default: `info`)

**SBOM Processing**:
- `SBOM_WORKERS`: Number of SBOM extraction workers (default: `3`)
- `SBOM_QUEUE_SIZE`: Work queue size (default: `100`)

---

## Build

### Prerequisites

- Go 1.21+
- Access to Kubernetes cluster (for testing)
- Containerd or Docker (for image access)

### Build Binary

```bash
go build -o bin/fortuna-agent ./cmd
```

### Build Docker Image

```bash
docker build -t fortuna-agent:latest -f Dockerfile .
```

Or using `nerdctl` (default namespace may **not** be what kubelet uses):

```bash
# From repository root (Dockerfile expects repo context for api/ + agent/)
nerdctl -n k8s.io build -t docker.io/library/fortuna-agent:latest -f agent/Dockerfile .
```

**Why `-n k8s.io`:** On many clusters the kubelet pulls images from the **containerd `k8s.io` namespace**. Building only in the default nerdctl namespace can leave `fortuna-agent:latest` invisible to Kubernetes or with a stale digest. The sync script `scripts/utils/push-images-to-workers.sh` exports `docker.io/library/fortuna-agent:latest`; match that tag when loading on nodes.

**Multi-node:** After building locally, import the same tarball on each node (the push script does this) or rebuild with `nerdctl -n k8s.io` on each host.

---

## Deployment

### Kubernetes DaemonSet

The Agent is deployed as a DaemonSet to run on every node.

**Recommended (repo root):** Use the manifests in the repository root. From repo root:

```bash
kubectl apply -f deploy/fortuna-rbac.yaml    # RBAC for core + agent
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

The `agent/deploy/` directory (rbac.yaml, daemonset.yaml) is for reference; the canonical deployment is `deploy/fortuna-agent-daemonset.yaml` and `deploy/fortuna-rbac.yaml` at repo root. See [deploy/README.md](../../deploy/README.md).

#### Verify deployment

```bash
# Check pods
kubectl get pods -l app=fortuna-agent -n fortuna

# Check logs
kubectl logs -l app=fortuna-agent -n fortuna
```

### Local Development

1. **Set Environment Variables**:
```bash
export CORE_GRPC_ENDPOINT=localhost:9090
export NODE_NAME=local-node
export TLS_ENABLED=false
export LOG_LEVEL=debug
```

2. **Run**:
```bash
go run cmd/main.go
```

---

## Data Flow

### End-to-End Process

```
1. Pod Created on Node
   ↓
2. Agent Detects Pod (Local Pod Watcher)
   ↓
3. Agent Enqueues Pod to SBOM Queue (Asynchronous)
   ↓
4. SBOM Worker Extracts SBOM:
   - Mounts container filesystem
   - Runs OS-aware parsers
   - Extracts packages with PURLs
   - Collects OS metadata
   ↓
5. Agent Sends SBOM to Core via gRPC
   ↓
6. Core Stores SBOM and Triggers CVE Matching
```

### SBOM Extraction Process

1. **Image Access**: Access container image via containerd socket
2. **Filesystem Mount**: Mount container root filesystem
3. **OS Detection**: Detect OS type (Debian, Alpine, etc.)
4. **Parser Selection**: Select relevant parsers based on OS
5. **Package Extraction**: Extract packages with versions
6. **PURL Generation**: Generate Package URLs for all components
7. **Metadata Collection**: Collect OS name, version, architecture
8. **SBOM Assembly**: Create SBOM JSON structure
9. **Send to Core**: Transmit via gRPC

---

## Security

### RBAC Permissions

Agent requires **read-only** permissions (pods, nodes, namespaces, serviceaccounts, RBAC resources). See `deploy/fortuna-rbac.yaml` at repo root for the full RBAC used in deployment.

### mTLS

- **Client Certificate**: Agent authenticates to Core
- **CA Verification**: Validates Core server certificate
- **Secure Channel**: All gRPC communication encrypted

### Least Privilege

- No write permissions
- No access to secrets
- Only reads pod/node information

---

## Troubleshooting

### Check Agent Logs

```bash
kubectl logs -l app=fortuna-agent -n fortuna
```

### Verify RBAC Permissions

```bash
kubectl auth can-i list pods \
  --as=system:serviceaccount:fortuna:fortuna-agent \
  -n fortuna
```

### Check Core Connectivity

```bash
# From agent pod
kubectl exec -it <agent-pod> -n fortuna -- \
  wget -O- http://fortuna-core.fortuna.svc.cluster.local:8080/health
```

### Verify Containerd Access

```bash
# Check containerd socket
kubectl exec -it <agent-pod> -n fortuna -- \
  ls -la /run/containerd/containerd.sock
```

### Common Issues

**Issue**: Falco alerts not reaching Core / empty `runtime_events` for a pod
- **Solution**: Set `FALCO_EVENTS_ENABLED=true` and mount host `/var/log/falco` (see `deploy/fortuna-agent-daemonset.yaml`). Ensure Falco writes `events.jsonl` on that node. The agent resolves `pod_uid` from `k8s.pod.name` + namespace via **list** if **get** is denied by RBAC. If the JSONL file is huge, the reader **starts at EOF** on first run (new alerts only); rotate or truncate the file on the host if you need a clean slate.

**Issue**: Agent `OOMKilled` when Falco is enabled
- **Solution**: DaemonSet uses higher memory limits and optional `SBOM_WORKERS=1` to reduce peak usage; ensure the deployed manifest matches `deploy/fortuna-agent-daemonset.yaml`.

**Issue**: Agent can't connect to Core
- **Solution**: Check `CORE_GRPC_ENDPOINT` and network policies

**Issue**: SBOM extraction fails
- **Solution**: Verify containerd socket access and image availability

**Issue**: mTLS handshake fails
- **Solution**: Verify certificates are mounted correctly

**Issue**: Pods not detected
- **Solution**: Check node name matches and RBAC permissions

---

## Performance

### Resource Usage

- **CPU**: 50m-500m (configurable)
- **Memory**: 256Mi-2Gi (configurable)
- **Workers**: 3 workers by default (configurable)

### Optimization

- **Async Queue**: Prevents blocking during slow SBOM extraction
- **Parallel Processing**: Multiple workers process pods concurrently
- **OS-Aware Parsing**: Only runs relevant parsers
- **Efficient Watchers**: Node-specific pod filtering

---

## Related Documentation

- [Architecture](../../docs/02-architecture/ARCHITECTURE.md)
- [Production Deployment](../../docs/05-operations/PRODUCTION_DEPLOYMENT.md)
- [Core README](../core/README.md)

---

**Version**: 1.0.0  
**Last Updated**: 2026-01-06
