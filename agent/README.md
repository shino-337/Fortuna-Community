# Fortuna Agent

Fortuna Agent runs as a DaemonSet on every Kubernetes node. It extracts SBOMs from the images of the pods on its node, syncs cluster inventory, reports Pod Detail data and forwards runtime events to Fortuna Core. It does **not** match CVEs; Core does that.

---

## Overview

### Key Responsibilities

- **SBOM extraction**: detect pods on the local node and extract an SBOM for each container image through an asynchronous work queue, then send it to Core over gRPC with mTLS
- **Inventory sync**: list Pods, ServiceAccounts, RBAC objects, Deployments and ReplicaSets and send them to Core over HTTP with a collection receipt ([Inventory](../docs/reference/INVENTORY.md))
- **Pod Detail**: read processes, sockets and network counters for local pods from host `/proc`, and CPU/memory usage from `metrics.k8s.io`
- **Runtime evidence**: forward Falco or file-based runtime events and report producer coverage ([Runtime evidence](../docs/reference/RUNTIME_EVIDENCE.md))
- **Parsers**: dpkg, apk, rpm, npm, pip, gomod, gobinary, maven, cargo, ruby (Gemfile.lock), nuget and distroless, with a Syft fallback for images where they find little

---

## Architecture

### Layout

```
agent/
├── cmd/
│   ├── main.go               # Agent entry point
│   └── image-export/         # Credential-less sidecar that alone holds the containerd socket
├── internal/
│   ├── watcher/, k8s/        # Local pod watcher and Kubernetes client
│   ├── sbom/                 # SBOM work queue and processor
│   ├── client/               # mTLS gRPC client to Core
│   ├── syncer/, collector/   # Inventory collection and HTTP sync
│   ├── corehttp/             # HTTP delivery to Core with per-Agent credentials
│   ├── poddetail/            # Host /proc readers and Pod Detail reporter
│   ├── runtime/              # Falco/file readers, coverage, lifecycle, source health, eBPF scaffold
│   └── cluster/, config/, converter/, retry/
├── pkg/
│   └── sbom/
│       ├── extractor/        # Package parsers and Syft adapter
│       ├── imageexport/      # Client for the image-export socket
│       └── signatures/       # Distroless binary signatures
└── Dockerfile
```

---

## Features

### 1. Local Pod Detection

- **Node-Specific**: Only monitors pods on the local node
- **Real-Time**: Uses Kubernetes Informers for immediate pod detection
- **Efficient**: Filters pods by node name to reduce API calls

### 2. SBOM Extraction

- **Parsers** (`pkg/sbom/extractor/`):
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
- **Syft fallback**: distroless or unknown images with few packages also get a Syft pass (`SBOM_USE_SYFT_FALLBACK`)
- **PURL Support**: Generates Package URLs (PURL) for all components
- **Metadata Extraction**: OS name, version, architecture

### 3. Asynchronous Work Queue

- **Non-Blocking**: Pod detection continues while SBOM extraction runs
- **Parallel Processing**: Multiple workers process pods concurrently
- **Configurable Workers**: 2 workers by default; the bundled manifest sets `SBOM_WORKERS=1`
- **Queue Management**: Prevents memory buildup with bounded queue

### 4. gRPC Communication

- **mTLS Support**: Mutual TLS for secure communication with Core
- **Automatic Reconnection**: Handles network interruptions
- **Heartbeat**: Periodic health checks
- **Error Handling**: Retry logic for transient failures

---

## Configuration

The most common settings are below; every setting with its default is in the [configuration reference](../docs/reference/CONFIGURATION.md#agent).

**Core Connection**:
- `CORE_GRPC_ENDPOINT`: Core gRPC endpoint (default `fortuna-core.fortuna.svc.cluster.local:9090`)
- `CORE_HTTP_ENDPOINT`: Core HTTP endpoint for inventory and runtime ingest (default `http://fortuna-core.fortuna.svc.cluster.local:8080`)
- `FORTUNA_AGENT_TOKEN_FILE`: Per-Agent HTTP token file ([scoped Agent credentials](../deploy/scoped-agent-credentials/README.md))
- `FORTUNA_INGEST_TOKEN`: Shared HTTP ingest token, used only when Core has no per-Agent credential registry

**Agent Identity**:
- `NODE_NAME`: Kubernetes node name (set from the downward API)
- `AGENT_ID`: Agent identifier (default `<NODE_NAME>-agent`)
- `CLUSTER_ID`, `CLUSTER_NAME`: Fixed cluster identity; discovered from the API server when unset

**TLS/mTLS**:
- `TLS_ENABLED`: Enable mTLS (default: `true`)
- `TLS_CERT_PATH`: Client certificate path (default: `/etc/fortuna/tls/client/tls.crt`)
- `TLS_KEY_PATH`: Client private key path (default: `/etc/fortuna/tls/client/tls.key`)
- `TLS_CA_CERT_PATH`: CA certificate path (default: `/etc/fortuna/tls/client/ca.crt`)

**Image access (SBOM)**:
- `IMAGE_EXPORT_SOCKET`: Unix socket of the `image-export` container (bundled manifest: `/run/fortuna-image-export/export.sock`). When unset, the Agent opens `CONTAINERD_SOCKET` itself.
- `CONTAINERD_SOCKET`, `CONTAINERD_NAMESPACE`: containerd socket and namespace, used by `image-export` (defaults `/run/containerd/containerd.sock`, `k8s.io`)
- `SBOM_PREFER_REGISTRY=1`: skip containerd and pull images anonymously from the registry

**Logging**:
- `LOG_LEVEL`: Log level (default: `info`)

**SBOM Processing**:
- `SBOM_WORKERS`: Number of SBOM extraction workers (default: `2`; the bundled manifest uses `1`)

**Inventory and runtime**:
- `SYNC_INTERVAL` (default `30s`; the bundled manifest uses `5m`), `HEARTBEAT_INTERVAL`, `WATCH_NAMESPACE`
- `FALCO_EVENTS_ENABLED`, `FALCO_EVENTS_PATH`, `FALCO_DELIVERY_STATE_PATH`
- `EBPF_ENABLED`, `EBPF_SIMULATE`: experimental no-op eBPF scaffold; the bundled manifest grants it no capabilities. `EBPF_SIMULATE` emits synthetic events; never use them as evidence.

---

## Build

### Prerequisites

- Go 1.26+ (see `go.mod`)
- Access to Kubernetes cluster (for testing)
- Containerd or Docker (for image access)

### Build Binary

```bash
go build -o bin/fortuna-agent ./cmd
```

### Build Docker Image

```bash
docker build -t fortuna-agent:dev -f Dockerfile .
```

Or using `nerdctl` (default namespace may **not** be what kubelet uses):

```bash
# From repository root (Dockerfile expects repo context for api/ + agent/)
nerdctl -n k8s.io build -t docker.io/library/fortuna-agent:dev -f agent/Dockerfile .
```

**Why `-n k8s.io`:** On many clusters the kubelet pulls images from the **containerd `k8s.io` namespace**. Building only in the default nerdctl namespace can leave the local image invisible to Kubernetes or with a stale digest. For release installs, prefer the published package image `ghcr.io/shino-337/fortuna-community/fortuna-agent:v1.0.0`.

**Multi-node:** After building locally, import the same tarball on each node (the push script does this) or rebuild with `nerdctl -n k8s.io` on each host.

---

## Deployment

### Kubernetes DaemonSet

The Agent is deployed as a DaemonSet to run on every node.

Use the manifests in the repository root. From repo root:

```bash
kubectl apply -f deploy/fortuna-rbac.yaml    # RBAC for core + agent
kubectl apply -f deploy/fortuna-agent-daemonset.yaml
```

See [deploy/README.md](../deploy/README.md) for every manifest and its settings.

#### Verify deployment

```bash
# Check pods
kubectl get pods -l app.kubernetes.io/component=agent -n fortuna

# Check logs
kubectl logs -l app.kubernetes.io/component=agent -n fortuna -c agent
```

### Local Development

1. **Set Environment Variables**:
```bash
export CORE_GRPC_ENDPOINT=localhost:9090
export CORE_HTTP_ENDPOINT=http://localhost:8080
export NODE_NAME=local-node
export TLS_ENABLED=false
export LOG_LEVEL=debug
```

Without TLS and a per-Agent certificate, Core refuses the Agent's gRPC writes (SBOMs); inventory sync over HTTP still works with a token. To exercise SBOM delivery locally, provision a client certificate as in [per-Agent mTLS](../deploy/scoped-agent-credentials/MTLS.md).

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
   - Exports and unpacks the image
   - Runs OS-aware parsers
   - Extracts packages with PURLs
   - Collects OS metadata
   ↓
5. Agent Sends SBOM to Core via gRPC
   ↓
6. Core Stores SBOM and Triggers CVE Matching
```

### SBOM Extraction Process

1. **Image Access**: Ask the `image-export` container for the image archive (it alone holds the containerd socket), or pull from the registry
2. **Layer Unpacking**: Unpack image layers into a scratch filesystem
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

The Agent reads pods, nodes, namespaces, ServiceAccounts, workloads, events and RBAC resources cluster-wide. It has no write verbs, no `pods/exec` and no `nodes/proxy` (which the kubelet also accepts for exec): Pod Detail reads processes and sockets from the host `/proc` mount and CPU/memory usage from the `metrics.k8s.io` API. Without metrics-server, the usage columns stay empty and the Agent logs why. `scripts/verify/test-agent-privileges.py` fails CI if any of these grants come back. See `deploy/fortuna-rbac.yaml` for the exact rules.

### mTLS

- **Client Certificate**: Agent authenticates to Core
- **CA Verification**: Validates Core server certificate
- **Secure Channel**: All gRPC communication encrypted

### Node privileges

Both containers drop every Linux capability and run with a read-only root filesystem and no privilege escalation; the pod uses no host namespaces. The `agent` container reads host `/proc` and the Falco log directory and writes only its state directory. The containerd socket is mounted only into the credential-less `image-export` container. [Agent privileges](../docs/reference/SECURITY.md#agent-privileges) lists each privilege, what a compromise would give an attacker, and the CI gate that keeps the list from growing.

---

## Troubleshooting

### Check Agent Logs

```bash
kubectl logs -l app.kubernetes.io/component=agent -n fortuna -c agent
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
kubectl exec -it <agent-pod> -n fortuna -c agent -- \
  curl -s http://fortuna-core.fortuna.svc.cluster.local:8080/healthz
```

### Verify Image Access

```bash
# Each export is logged by the image-export container
kubectl logs <agent-pod> -n fortuna -c image-export --tail=20
```

### Common Issues

**Issue**: Falco alerts not reaching Core / empty `runtime_events` for a pod
- **Solution**: Set `FALCO_EVENTS_ENABLED=true` and mount host `/var/log/falco` (see `deploy/fortuna-agent-daemonset.yaml`). Ensure Falco writes `events.jsonl` on that node. The agent resolves `pod_uid` from `k8s.pod.name` + namespace via **list** if **get** is denied by RBAC. On first use the reader skips records already in the file and starts with new alerts; later restarts resume from the durable cursor in `FALCO_DELIVERY_STATE_PATH`. **Never truncate the Falco log or delete the state file to retry ingestion**: that discards evidence that has not been sent yet. See [Preserve Falco delivery state](../docs/operations/RUNTIME_SENSORS.md#preserve-falco-delivery-state).

**Issue**: Agent `OOMKilled` when Falco is enabled
- **Solution**: DaemonSet uses higher memory limits and optional `SBOM_WORKERS=1` to reduce peak usage; ensure the deployed manifest matches `deploy/fortuna-agent-daemonset.yaml`.

**Issue**: Agent can't connect to Core
- **Solution**: Check `CORE_GRPC_ENDPOINT` and network policies

**Issue**: SBOM extraction fails
- **Solution**: Check the `image-export` container logs (above). "image not found in containerd" means the image is not on this node; the Agent then falls back to an anonymous registry pull

**Issue**: mTLS handshake fails
- **Solution**: Verify certificates are mounted correctly

**Issue**: Pods not detected
- **Solution**: Check node name matches and RBAC permissions

---

## Performance

### Resource Usage

- **CPU**: request 100m, limit 1 core (bundled manifest)
- **Memory**: request 1Gi, limit 6Gi (bundled manifest; SBOM extraction of large images is the main consumer)
- **Workers**: 2 SBOM workers by default, 1 in the bundled manifest

### Optimization

- **Async Queue**: Prevents blocking during slow SBOM extraction
- **Parallel Processing**: Multiple workers process pods concurrently
- **OS-Aware Parsing**: Only runs relevant parsers
- **Efficient Watchers**: Node-specific pod filtering

---

## Related Documentation

- [Architecture](../docs/architecture/ARCHITECTURE.md)
- [Production Deployment](../docs/operations/PRODUCTION_DEPLOYMENT.md)
- [Configuration reference](../docs/reference/CONFIGURATION.md)
- [Core README](../core/README.md)
