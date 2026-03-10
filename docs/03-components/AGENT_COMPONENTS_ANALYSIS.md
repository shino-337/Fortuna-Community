# Fortuna Agent Component Analysis

This report enumerates the Agent codebase by module, describes the logic each module implements,
and calls out modules that are unused, legacy, or partially wired.

---

## 1) Entry Point

### `agent/cmd/main.go`
**Purpose**
- Agent bootstrap: config, K8s client, gRPC client, SBOM extraction, local pod watcher, auto‑sync, heartbeat.

**Key logic**
- Loads config (`internal/config`).
- Creates in‑cluster K8s client.
- Starts **auto full sync** to Core via HTTP (`internal/syncer`).
- Creates **mTLS gRPC client** to Core (`internal/client`).
- Registers agent and sends periodic heartbeat.
- Starts **LocalPodWatcher** (node‑scoped) and **SBOM work queue**.
- Processes existing pods on the node at startup.

**Notes**
- SBOM extraction is **node-local**, not cluster‑wide.
- CVE matching **never** happens in Agent (explicitly delegated to Core).

---

## 2) Internal Modules

### `internal/config`
**Purpose**
- Reads env vars for runtime configuration.

**Key fields**
- `CLUSTER_ID`, `CORE_GRPC_ENDPOINT`, `CORE_HTTP_ENDPOINT`, `SYNC_INTERVAL`
- `TLS_*` for mTLS
- `WATCH_NAMESPACE`, `KUBECONFIG`

### `internal/k8s`
**Purpose**
- Creates in‑cluster or kubeconfig‑based `kubernetes.Clientset`.

### `internal/client` (gRPC)
**Purpose**
- mTLS gRPC client to Core.

**Key logic**
- `Connect()` with client cert + CA pool.
- RPCs: `SendSBOMFinding`, `RegisterAgent`, `Ping`.

**Notes**
- `grpc_client_mtls.go` is the active implementation.
- `grpc_client.go.OLD` and `grpc_client_combined.go.OLD` are legacy and unused.

### `internal/sbom`
**Purpose**
- `Processor` extracts SBOM for each container and sends findings to Core.

**Key logic**
- Verifies pod is on local node.
- Extracts SBOM using `pkg/sbom/extractor`.
- Converts to proto and sends `SendSBOMFinding` RPC.

### `internal/sbom/queue.go`
**Purpose**
- Work queue for async SBOM processing.

**Key logic**
- Avoids blocking pod informer by distributing work to workers.

### `internal/watcher`
**Purpose**
- Pod event watchers.

**Key logic**
- `LocalPodWatcher` (node‑scoped informer) is actively used.
- `PodWatcher` (namespace‑scoped watch) is legacy (used only by Collector).

### `internal/syncer`
**Purpose**
- **Auto full‑sync** to Core via HTTP `/api/v1/agent/sync`.

**Key logic**
- Lists pods, service accounts, roles, bindings, cluster roles/bindings.
- Builds `linkedPods` mapping for SAs.
- POSTs full sync periodically based on `SYNC_INTERVAL`.

### `internal/collector` (UNUSED)
**Purpose**
- Legacy streaming inventory collector using gRPC `StreamInventory`.

**Status**
- Not wired in `cmd/main.go`.
- `collector_new.go` references `client.NewNewGRPCClient`, which does not exist.
- `converter` + namespace watchers are only used by this legacy collector.

### `internal/converter` (UNUSED)
**Purpose**
- Converts K8s resources into `InventoryItem` messages.

**Status**
- Only used by `internal/collector`, which is not wired.

### `internal/retry` (UNUSED)
**Purpose**
- Generic exponential backoff helper.

**Status**
- No call sites in Agent code.

---

## 3) Package Modules

### `pkg/sbom/extractor`
**Purpose**
- Extracts packages from container images using custom parsers.

**Key logic**
- Local‑first image fetch: containerd socket → remote registry fallback.
- Detect OS (dpkg/apk/rpm), run parsers (apk, dpkg, rpm, npm, pip, gomod).
- Deduplicate packages and emit `RawSBOM` (digest + package list).

### `pkg/models` (UNUSED)
**Purpose**
- Local SBOM/CVE model definitions.

**Status**
- Agent does not persist data locally; models are unused.

### `pkg/types`
**Purpose**
- Shared types for inventory/agent payloads.

**Status**
- Used indirectly by legacy inventory streaming paths.

---

## 4) Active Data Flows

### SBOM path
1) `LocalPodWatcher` sees pod Running on node.  
2) Enqueue pod → `SBOM Work Queue`.  
3) `Processor` → `Extractor` → packages → gRPC `SendSBOMFinding`.  
4) Core handles CVE matching.

### Auto‑sync path
1) `Syncer` lists K8s resources on schedule.  
2) POST `/api/v1/agent/sync` to Core.  
3) Core stores pods/RBAC and triggers historical risk evaluation.

---

## 5) Unused or Legacy Code

- `internal/collector/*` and `internal/converter/*`: legacy inventory streaming.
- `internal/retry`: unused helper.
- `pkg/models` in agent: not used in runtime.
- `internal/client/*OLD`: legacy gRPC implementations.

