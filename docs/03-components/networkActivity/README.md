# Network Activity Component

## Overview

The Network Activity component provides **per-pod network connection monitoring** and **cluster-wide network visibility**. It collects TCP/UDP connection snapshots from pods and presents them through Pod Detail views and a dedicated Network Activity dashboard page.

**Current scope:** Snapshot-based connection monitoring via Agent pod detail collection.
**Future roadmap:** eBPF kernel hooks, Redis Streams, TimescaleDB for real-time flow tracking (not yet implemented).

## Architecture

### Data Flow

```
Host: /host/proc (or exec ss/netstat)
    ↓
Fortuna Agent (poddetail Reporter)
    ↓ POST /api/v1/agent/pod-network-connections
Fortuna Core (ingest + upsert)
    ↓
PostgreSQL (pod_network_connections)
    ↓ GET /runtime/pods/:uid/network
    ↓ GET /api/v1/runtime/network-activity
Dashboard (Pod Detail / Network Activity page)
```

### Two Collection Modes (Agent)

Controlled by env `POD_DETAIL_RUNTIME_SOURCE` (see `agent/internal/poddetail/reporter.go`):

| Mode | Source | When |
|------|--------|------|
| **host** | `/proc/<pid>/net/tcp\|udp` on host, mapped via cgroup → container → pod | `POD_DETAIL_RUNTIME_SOURCE=host` or `true` or `1` |
| **exec** | `ss -tunap` / `netstat -tunap` inside each container | `POD_DETAIL_RUNTIME_SOURCE=exec` |
| **auto** (default) | Host mode if `/host/proc` readable, else exec | Not set or `auto` |

**Note:** `bytesSent`/`bytesRecv` are **snapshot queue values** from `/proc/net/*`, not cumulative session byte counters.

## Processing Flow

### Ingest (Agent → Core)

- **Endpoint:** `POST /api/v1/agent/pod-network-connections`
- **Bucketing:** All connections in a batch get a 5-minute UTC bucket (`bucket_5m`)
- **Upsert:** Key = `(signature, bucket_5m)` — updates snapshot within same bucket, preserves `created_at`
- **Protocol:** Normalized to lowercase
- **Side effects:** May generate `RuntimeEvent` for anomaly queue spike (R5); broadcast Pod Detail via WebSocket

### Read (Dashboard/API)

**Per-Pod:**
- `GET /runtime/pods/:uid/network` — default 24h (`sinceMinutes=1440`), limit 500 (max 2000), sorted by `bucket_5m DESC`

**Per-Pod Top Destinations:**
- `GET /runtime/pods/:uid/network/top-destinations` — limit 15 (max 50), returns `destIp`, `destPort`, `protocol`, `observationCount`

**Cluster-Wide:**
- `GET /api/v1/runtime/network-activity` — requires `cluster` param
- **Views:** `connections` (default), `pods` (grouped), `destinations` (top dest cluster-wide with `destWorkloadName`), `talkers` (top source pods)
- **Filters:** `namespace`, `q` (search by pod/IP/port), `sinceMinutes`, `page`, `pageSize` (max 200)

### Retention

- Job: `PodNetworkRetentionJob` — deletes data older than retention period
- Env vars:
  - `POD_NETWORK_RETENTION_HOURS` (default 24)
  - `POD_NETWORK_CLEANUP_INTERVAL` (default 10m)
  - `POD_NETWORK_CLEANUP_BATCH` (20000)
  - `POD_NETWORK_CLEANUP_ROUNDS` (3)
  - `POD_NETWORK_CLEANUP_INITIAL_DELAY` (optional, e.g. 2m)

## Technical Details

### Database Schema

**Table: `pod_network_connections`**

| Column | Type | Description |
|--------|------|-------------|
| id | bigint | Primary key |
| pod_uid | varchar | Pod identifier |
| cluster_id | varchar | Cluster identifier |
| namespace | varchar | Kubernetes namespace |
| source_ip | varchar | Source IP address |
| source_port | int | Source port |
| dest_ip | varchar | Destination IP address |
| dest_port | int | Destination port |
| protocol | varchar | tcp/udp (lowercase) |
| state | varchar | Connection state (established, etc.) |
| bytes_sent | bigint | TX queue snapshot (not cumulative) |
| bytes_recv | bigint | RX queue snapshot (not cumulative) |
| runtime_source | varchar | `host` or `exec` |
| signature | varchar | Unique connection identifier |
| bucket_5m | timestamp | 5-minute UTC bucket |
| observed_at | timestamp | Ingest time |
| created_at | timestamp | First observation |

**Indexes:** `(signature, bucket_5m)`, `(pod_uid)`, `(cluster_id, namespace)`, `(bucket_5m)`

### Key Code Paths

| Component | Path |
|-----------|------|
| Agent network collector | `agent/internal/poddetail/network_collector.go` |
| Agent host network | `agent/internal/poddetail/host_network.go`, `proc_net.go` |
| Core ingest handler | `core/internal/api/pod_detail_services_handlers.go` |
| Core network activity handler | `core/internal/api/network_activity_handler.go` |
| Retention job | `core/internal/scheduler/pod_network_retention_job.go` |
| Dashboard page | `dashboard/pages/NetworkActivity.tsx` |
| Dashboard Pod Detail | `dashboard/pages/PodDetail.tsx` (network tab) |

### Dashboard UI (Network Activity Page)

Four view modes:
1. **Connections** — raw connection list with pod/namespace/IP/port
2. **Pods** — grouped by pod with connection count and last observed
3. **Destinations** — cluster-wide top destinations with observation count, pod count
4. **Talkers** — top source pods by distinct destination count

Filters: cluster selector, namespace, search (pod/IP/port), time window.

## Known Gaps

| ID | Description | Priority | Status |
|----|-------------|----------|--------|
| R5 | Packet/throughput counters per flow (only queue snapshot available) | P2 | In Progress |
| NET-1 | eBPF kernel hooks for real-time flow capture | P2 | Roadmap |
| NET-2 | Redis Streams for short-TTL flow buffering | P3 | Roadmap |
| NET-3 | TimescaleDB for down-sampled historical flows | P3 | Roadmap |
| NET-4 | Real-time network graph visualization | P3 | Roadmap |
| NET-5 | Service ClusterIP resolution for `destWorkloadName` | P3 | Known |

## Related Documentation

- [Pod Detail Component](../podDetail/README.md) — Network connections per pod
- [Dashboard Component](../dashboard/README.md) — Network Activity page
