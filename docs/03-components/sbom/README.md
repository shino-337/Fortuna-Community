# SBOM Component

**Status:** Production Ready (MVP2)  
**Owners:** Core + Agent  
**Method:** Custom local-first extraction (zero external tools)  
**Performance:** ~30 seconds per image (first scan), <1 s cached

---

## Overview

The SBOM (Software Bill of Materials) component extracts a package inventory from every container image running in the cluster and matches it against vulnerability databases (OSV, NVD) to surface CVEs on the Risk Center dashboard.

Key properties:

- **Zero external dependencies** — no Syft, Trivy, or Grype binaries required; pure Go implementation.
- **Local-first image access** — reads from the node's containerd/Docker daemon first; falls back to the remote registry only when needed, eliminating Docker Hub rate limits for cached images.
- **Digest-based caching** — an SBOM is generated once per image digest and reused for every pod that shares the same image.
- **Event-driven CVE matching** — after an SBOM is persisted the `fortuna.sbom.created` NATS event triggers the CVE matcher worker asynchronously.
- **Immutable snapshots** — once finalized, an SBOM can only be mutated with an explicit context flag (`WithSBOMMutationAllowed`).

---

## Architecture

### Key Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| **SBOM Extractor** (Agent) | `agent/pkg/sbom/extractor/` | Pulls image layers, runs parsers (dpkg, apk, rpm, npm, pip, gomod, gobinary, distroless), generates PURLs |
| **SBOM Work Queue** (Agent) | `agent/internal/sbom/queue.go` | Async queue (buffer 30, default 2 workers) decouples pod detection from extraction |
| **Pod Watcher** (Agent) | `agent/internal/watcher/pod_watcher_local.go` | Kubernetes informer; enqueues Running pods for SBOM processing |
| **SBOM Processor** (Agent) | `agent/internal/sbom/processor.go` | Orchestrates extraction per container, sends `SBOMFinding` to Core via gRPC |
| **Core SBOM Handler** | `core/internal/grpc/handler_sbom.go` | Receives gRPC `SendSBOMFinding`, upserts `sboms` + `sbom_components`, publishes NATS event |
| **CVE Matcher Worker** | `core/pkg/cve/matcher/matcher.go`, `cve_matcher_worker.go` | Subscribes to `fortuna.sbom.created`, runs OSV bulk match + NVD fallback, persists `cve_matches` |
| **OSV Loader** | `core/pkg/cve/loader/osv_parser.go` | Ingests OSV JSON into `package_vulnerabilities` and `cves` tables |
| **NVD Client** | `core/pkg/cve/database/nvd/client.go` | Fallback keyword search against NVD REST API (`https://services.nvd.nist.gov/rest/json/cves/2.0`) |
| **Component Resolver** | (design — see Conflict Resolution) | Deduplicates and resolves multi-source components before matching |
| **SBOM Reconciler** | `core/pkg/reconciler/sbom_reconciler.go` | Cleans up orphaned SBOMs (with 2-hour grace period to avoid race with pod sync) |
| **Streaming VFS** | `agent/pkg/sbom/extractor/filesystem.go` | Indexed + selective-materialize mode to keep RAM low (`SBOM_FS_MODE=indexed`) |

### Data Flow

```
[Agent on node]
  Pod Watcher (informer) → detects Running pod
       ↓ (non-blocking enqueue)
  SBOM Work Queue (async, 2 workers default)
       ↓
  SBOM Processor → ExtractSBOM(image)
       │
       ├─ Try local daemon (containerd/Docker socket)
       │   └─ Fallback: remote registry
       ↓
  Parse package databases
       ├─ /var/lib/dpkg/status  (Debian)
       ├─ /lib/apk/db/installed (Alpine)
       ├─ /var/lib/rpm           (Red Hat)
       ├─ package.json / package-lock.json (Node)
       ├─ *.dist-info/METADATA  (Python)
       ├─ go.mod / go.sum       (Go modules)
       ├─ debug/buildinfo       (Go binaries)
       └─ distroless signatures (heuristic)
       ↓
  Generate PURLs → convertToProto (pod.UID, image, packages)
       ↓
  gRPC SendSBOMFinding → Core
       ↓
[Core]
  handler_sbom: upsert sboms + sbom_components (finalized, version++)
       ↓
  NATS Publish "fortuna.sbom.created"
    { sbom_id, pod_uid, namespace, container_name, image_digest,
      components_snapshot[]: {name, version, purl} }
       ↓
  CVE Matcher Worker
       ├─ Idempotency gate: (sbom_id, sbom.version, mirror_state.osv.version)
       ├─ Bulk OSV match (Postgres package_vulnerabilities)
       ├─ Go module matcher (prefix + alias)
       ├─ Go stdlib matcher
       └─ NVD fallback (heuristic SBOM + whitelist only)
       ↓
  Persist cve_matches + create vulnerability insights
       ↓
[Dashboard / API]
  GET /api/v1/sbom            — list SBOMs
  GET /api/v1/sbom/:podUid    — SBOM detail by pod UID
```

---

## Processing Flow

### SBOM Extraction Flow

1. Agent Pod Watcher detects a Running pod on the local node (`spec.nodeName = NODE_NAME`).
2. Pod is enqueued into the SBOM Work Queue (channel buffer 30). If the queue is full the pod is dropped (logged as `Queue full`).
3. A worker dequeues the pod, iterates its containers, and calls `ExtractSBOM(image)` for each.
4. The extractor tries `daemon.Image(ref)` (local containerd/Docker) first. On failure it falls back to `remote.Image(ref)`.
5. Image layers are streamed through the VFS builder (materialize or indexed mode). Parsers run against the virtual filesystem.
6. Packages are deduplicated by `(Type, Name, Version)`. PURLs are generated per ecosystem.
7. `SBOMFinding` (including `pod_uid`, `source`, `confidence`) is sent to Core via gRPC.
8. Core upserts `sboms` (keyed by `pod_uid`) and `sbom_components`, then publishes the NATS event.

### CVE Matching Flow (Matcher V2)

```
Event: fortuna.sbom.created
  ↓
Load Snapshot (components_snapshot from event — avoids soft-delete race)
  ↓
PURL Validation Filter (syntax → ecosystem consistency → name → version)
  ↓
Component Resolver (group by canonical identity, pick highest-priority source)
  ↓
Matching Engine
  ├─ Parse PURL → ecosystem + name + version
  ├─ Alias resolution (pkg:golang/… → pkg:go/…)
  ├─ Bulk query: SELECT * FROM osv_packages JOIN osv_vulnerabilities WHERE package IN (…)
  └─ Version constraint matching (semver, distro-specific)
  ↓
NVD Fallback (heuristic SBOMs only, whitelist: coredns, etcd, kube-*, openssl, runc)
  ↓
Persist cve_matches (ON CONFLICT DO NOTHING)
  ↓
Build Insights → Risk Engine → Dashboard
```

**Idempotency:** The worker only runs matching once per `(sbom_id, sbom.version, mirror_state.osv.version)`. Duplicate events or retries are skipped via `EnsureMatchRun()`.

**Noise reduction:** `gobinary-main` components are non-matchable (priority 0). Low-trust PURLs and shadowed components are skipped.

### Async Queue Processing

The Agent uses a work-queue pattern to decouple pod detection from SBOM extraction:

- **Queue size:** 30 pods (buffered channel).
- **Workers:** 2 by default (configurable via `SBOM_WORKERS`).
- **Duplicate prevention:** Tracks active pods by `namespace/name`.
- **Fallback:** If the queue is full, the pod can be processed synchronously.

Performance improvement over synchronous mode:

| Metric | Synchronous | Async Queue |
|--------|-------------|-------------|
| Pod detection latency | 2–3 min | < 1 s |
| Startup (15 pods) | 30–45 min | < 1 s |
| Throughput | 1 pod / 3 min | 2+ pods / 3 min |

### Control Plane SBOM Handling

Control-plane pods (kube-apiserver, etcd, CoreDNS, etc.) use distroless/scratch images with no package manager. Fortuna handles them via:

1. **Distroless signature DB** (`agent/pkg/sbom/signatures/distroless.json`) — allowlist of known binaries with canonical names, PURLs, label keys, and optional `digestMap` entries.
2. **Version resolution order:** digestMap → image tag → OCI label (`org.opencontainers.image.version`, `io.k8s.display-version`) → `ref.name` label fallback.
3. **Synthetic SBOM:** A single `pkg:generic/<name>@<version>` component is emitted with `source=distroless-heuristic` and `confidence=medium`.
4. **CVE matching:** OSV query with `ecosystem=generic` first; then NVD keyword fallback if whitelisted.
5. **Go binary analyzer** (`agent/pkg/sbom/extractor/gobinary.go`): scans `/bin`, `/usr/bin`, `/usr/local/bin`, `/usr/sbin`, `/sbin`, `/app`, `/` for Go binaries, extracts `debug/buildinfo` → emits `pkg:go/…` dependencies and main module.

### Distroless Image Handling

The distroless parser only emits components when the binary is in the signature DB. This prevents "junk" entries (setpriv, ln, mkdir, etc.) from polluting SBOMs of images that already have OS package manager output.

Tests: `TestDistrolessParser`, `TestDistrolessParserJunkNotEmitted`, `TestDistrolessParserSkipsJunk`, `TestDistrolessParserUsesSignatures`.

### Streaming VFS (RAM Optimization)

The `SBOM_FS_MODE=indexed` mode (Phase 2, implemented) reduces peak RAM by 60–97 %:

- **Pass 1:** Index-only scan of tar headers — builds path index without reading content.
- **Pass 2:** Selective extract of files matching a suffix allowlist (dpkg status, apk installed, go.mod, package.json, etc.).
- **Lazy spool:** Large files (Go binaries) are read on demand via temp-file spool with `ReadAt`.
- **Env:** `SBOM_FS_MODE`, `SBOM_FS_MAX_TOTAL_BYTES` (1 GiB default), `SBOM_FS_MAX_FILE_BYTES` (64 MiB), `SBOM_FS_SPOOL_DIR`, `SBOM_FS_SKIP_PATH_PREFIXES`, `SBOM_FS_METRICS`.

---

## Technical Details

### PURL Validation Policy

PURLs are the **source of truth** for ecosystem, name, and version in the matcher. Validation is layered:

| Layer | Check | On failure |
|-------|-------|------------|
| 1. Syntax | Must conform to `pkg:<type>/<name>@<version>` | Regenerate from (type, name, version) |
| 2. Ecosystem consistency | `go` → Go, `deb` → Debian, `apk` → Alpine | Normalize ecosystem, log warning |
| 3. Name | Go: must be full module path with domain | Mark `invalid_name=true` |
| 4. Version | Go: semver or pseudo-version; OS: distro format | Downgrade confidence |

**Go normalization:** `pkg:golang/…` is always treated as `pkg:go/…`.

**Trust levels:** `HIGH` (agent-generated, valid), `MEDIUM` (normalized), `LOW` (fallback/generated).

### Component Identity

- **PURL** is the canonical identifier for matching.
- **CPE** is used only for NVD queries (generated from component name).
- **Image digest** keys the SBOM cache.
- Matching uses `components_snapshot` from the NATS event (not the DB) to avoid race conditions with soft-delete.
- When building insights, the worker supplements the DB `componentMap` with snapshot entries for any `ComponentName` not yet in DB.

### Conflict Resolution

Multiple sources can report the same logical package (gomod, gobinary, OS, heuristic). The Component Resolver produces a single canonical set:

**Source priority:**

| Source | Priority | Notes |
|--------|----------|-------|
| `gobinary` | 100 | Runtime build info — highest trust |
| `gomod` | 80 | Static dependency |
| `os` (dpkg/apk/rpm) | 60 | OS package manager |
| `heuristic` | 20 | Distroless synthetic |
| `gobinary-main` | 0 | Non-matchable (inventory only) |

**Algorithm:** Group by `(ecosystem, normalized_name)` → filter invalid/missing-version → remove non-matchable → sort `(priority DESC, confidence DESC)` → pick winner → mark rest as `shadowed`.

### Event Replay Contract

The `fortuna.sbom.created` event is the trigger for deterministic CVE processing.

**Required fields:** `event_id`, `timestamp`, `schema_version` (currently `v1`), `components_snapshot`.

**Snapshot canonical fields:** `purl`, `name`, `version`, `normalized_name`, `version_class` (STRICT|LOOSE|INVALID|UNKNOWN), `ecosystem`, `namespace`, `arch`, `source`, `trust_level`, `original_purl`, `purl_validated`.

**Replay guard:** `ClaimSBOMEvent(sbom_id, event_id, timestamp)` — atomic, last-write-wins by timestamp. Same timestamp + same event_id = idempotent skip.

### Database Schema

#### `sboms`

| Column | Type | Notes |
|--------|------|-------|
| `id` | SERIAL PK | |
| `pod_uid` | VARCHAR | Kubernetes pod UID (primary lookup key) |
| `pod_name` | VARCHAR | |
| `namespace` | VARCHAR | |
| `container_name` | VARCHAR | |
| `image_name` | VARCHAR | |
| `image_tag` | VARCHAR | |
| `image_digest` | VARCHAR UNIQUE | Cache key |
| `sbom_format` | VARCHAR | Default `cyclonedx-json` |
| `sbom_content` | JSONB | |
| `package_count` | INT | |
| `os_packages` | INT | |
| `language_packages` | INT | |
| `sbom_source` | VARCHAR | `parsers`, `distroless-heuristic`, `label-metadata` |
| `confidence` | VARCHAR | `low`, `medium`, `high` |
| `generator` | VARCHAR | Default `custom` |
| `status` | VARCHAR | `finalized` |
| `version` | INT | Incremented on upsert |
| `created_at` / `updated_at` / `deleted_at` | TIMESTAMPTZ | Soft delete |

#### `sbom_components`

| Column | Type | Notes |
|--------|------|-------|
| `id` | SERIAL PK | |
| `sbom_id` | INT FK → sboms | |
| `component_name` | VARCHAR | |
| `component_version` | VARCHAR | |
| `component_type` | VARCHAR | |
| `purl` | VARCHAR(512) | |
| `source` | VARCHAR | Parser that produced this component |
| `UNIQUE(sbom_id, purl)` | | Dedup key |

#### `cve_matches`

| Column | Type | Notes |
|--------|------|-------|
| `id` | SERIAL PK | |
| `sbom_id` | INT FK → sboms | |
| `component_id` | INT FK → sbom_components | |
| `package_name` | VARCHAR | |
| `cve_id` | VARCHAR(20) | |
| `severity` | VARCHAR(20) | |
| `cvss_score` | DECIMAL(3,1) | |
| `fixed_version` | VARCHAR | |
| `matched_by` | VARCHAR | `fortuna-core-cve-matcher` or `nvd-fallback` |
| `UNIQUE(sbom_id, package_name, cve_id)` | | Dedup |

#### `sbom_match_runs`

Idempotency table for matcher: `(sbom_id, version, mirror_version)`.

#### `package_vulnerabilities` / `cves`

OSV-loaded vulnerability data queried during matching.

#### `mirror_state`

Tracks OSV mirror version; bumping version triggers re-matching.

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/sbom` | List SBOMs (paginated, filter by `podName`, `namespace`) |
| GET | `/api/v1/sbom/:podUid` | SBOM detail by pod UID |
| GET | `/api/v1/sboms` | List SBOMs (alternate route) |
| GET | `/api/v1/sboms/{sbom_id}` | Get specific SBOM |
| GET | `/api/v1/sboms/by-digest/{digest}` | Get SBOM by image digest |
| GET | `/api/v1/sboms/{sbom_id}/components` | Get SBOM components |
| GET | `/api/v1/inventory/pods/:uid/sbom` | Dashboard pod detail SBOM route |

API returns 404 with `{ error, podUid, hint }` when no SBOM exists for the pod.

### Environment Variables

#### Agent

| Variable | Default | Description |
|----------|---------|-------------|
| `SBOM_ENABLED` | `true` | Enable SBOM extraction |
| `SBOM_WORKERS` | `2` | Number of async queue workers |
| `SBOM_PREFER_REGISTRY` | `false` | Prefer registry over local daemon |
| `SBOM_FS_MODE` | `materialize` | `indexed` for low-RAM streaming VFS |
| `SBOM_FS_MAX_TOTAL_BYTES` | `1GiB` | Content cache cap |
| `SBOM_FS_MAX_FILE_BYTES` | `64MiB` | Per-file cap |
| `SBOM_FS_SPOOL_DIR` | OS temp | Layer spool directory |
| `SBOM_FS_SKIP_PATH_PREFIXES` | *(empty)* | Comma-separated prefixes to skip |
| `SBOM_FS_METRICS` | on | `off` to suppress `[SBOM FS]` log line |

#### Core

| Variable | Default | Description |
|----------|---------|-------------|
| `FORTUNA_CVE_SOURCE` | `postgres` | CVE data source (OSV in Postgres) |
| `NVD_API_KEY` | *(empty)* | NVD API key (increases rate limit from 5→50 req/30 s) |
| `FORTUNA_NVD_DISABLED` | `false` | Disable NVD fallback entirely |

### Code Paths

| Area | Key Files |
|------|-----------|
| Extractor | `agent/pkg/sbom/extractor/extractor.go`, `filesystem.go` |
| Parsers | `agent/pkg/sbom/extractor/dpkg.go`, `apk.go`, `rpm.go`, `npm.go`, `pip.go`, `gomod.go`, `gobinary.go`, `distroless.go` |
| Signatures | `agent/pkg/sbom/signatures/distroless.json` |
| Queue | `agent/internal/sbom/queue.go` |
| Processor | `agent/internal/sbom/processor.go` |
| Watcher | `agent/internal/watcher/pod_watcher_local.go` |
| Agent main | `agent/cmd/main.go` |
| Core handler | `core/internal/grpc/handler_sbom.go` |
| API handlers | `core/internal/api/sbom_handlers.go` |
| Routes | `core/internal/api/routes_inventory.go` |
| Matcher | `core/pkg/cve/matcher/matcher.go` |
| Matcher worker | `core/pkg/cve/matcher/cve_matcher_worker.go` |
| NVD client | `core/pkg/cve/database/nvd/client.go` |
| CVE manager | `core/pkg/cve/database/manager.go` |
| OSV loader | `core/pkg/cve/loader/osv_parser.go` |
| Reconciler | `core/pkg/reconciler/sbom_reconciler.go` |

### NATS Subjects

| Subject | Publisher | Subscriber | Payload |
|---------|-----------|------------|---------|
| `fortuna.sbom.created` | Core (after DB commit) | CVE Matcher Worker | `sbom_id`, `pod_uid`, `namespace`, `container_name`, `image_digest`, `components_snapshot[]` |

JetStream is used for durability. `AckWait` is 2 minutes. Max retry is 5 (JetStream redelivery).

---

## Troubleshooting

### SBOM Not Generated for a Pod

**Step A — Identify pod and node:**

```bash
kubectl get pod -n <ns> <name> -o jsonpath='{.metadata.uid}'
kubectl get pod -n <ns> <name> -o jsonpath='{.spec.nodeName}'
```

**Step B — Check DB:**

```sql
SELECT id, pod_uid, pod_name, namespace, created_at, deleted_at
FROM sboms WHERE pod_uid = '<uid>';
```

- No rows → Agent never sent an SBOM for this pod.
- Row with `deleted_at` set → SBOM was soft-deleted (pod deleted or reconciler cleanup).

**Step C — Verify Agent runs on the pod's node:**

```bash
kubectl get pods -n fortuna -l app=fortuna-agent -o wide
```

Agent only processes pods on its own node. If the pod is on a different node, no SBOM will be created.

**Step D — Check Agent logs:**

```bash
kubectl logs -n fortuna -l app=fortuna-agent --tail=2000 | grep -iE 'SBOM|Queue|Extract|SendSBOM|fail|error'
```

Look for:
- `Queued pod <ns>/<name>` — pod was enqueued.
- `Queue full, dropping pod` — queue is full; increase `SBOM_WORKERS` or queue buffer.
- `SBOM extraction failed` — check image pull access.
- `SendSBOMFinding RPC failed` — gRPC/TLS issue to Core.

**Step E — Check Core logs:**

```bash
kubectl logs -n fortuna -l app=fortuna-core --tail=2000 | grep -iE '\[SBOM\]|sbom|fail|error'
```

Look for:
- `[SBOM] Received SBOM from agent=…`
- `[SBOM] Created new SBOM id=… for pod_uid=…`

### Common Causes

| Cause | Symptom | Fix |
|-------|---------|-----|
| Pod on node without Agent | No Agent log for this pod | Ensure Agent DaemonSet runs on all nodes |
| Queue full | Agent: `Queue full, dropping pod` | Increase `SBOM_WORKERS` (2→4) or buffer |
| Image pull failure | Agent: `SBOM extraction failed` | Check image registry access / credentials |
| gRPC failure | Agent: `SendSBOMFinding RPC failed` | Check Core reachability, mTLS certs |
| Pod recreated (new UID) | Old SBOM soft-deleted, new one pending | Wait for Agent to process the new pod |
| SBOM deleted by Reconciler | SBOM arrived before pod sync (race) | Fixed: 2-hour grace period in `cleanupOrphanedSBOMs` |

### Pod Has SBOM but Components Array is Empty

1. Query `sbom_components WHERE sbom_id = <id> AND deleted_at IS NULL` — if 0 rows, components were never inserted or were deleted.
2. Check Core log for `stored SBOM id=X with N components` — if N=0, Agent sent empty packages.
3. Check for soft-deleted components: `SELECT * FROM sbom_components WHERE sbom_id = <id>` (without `deleted_at IS NULL`).

### CVEs Not Showing for Control-Plane Pods

- Verify SBOM exists with a version (not `unknown`). If unknown, populate `digestMap` in `distroless.json` or ensure OCI labels are present.
- OSV `generic` ecosystem is mostly empty — NVD fallback is the primary source for control-plane CVEs.
- Component name must match NVD whitelist after normalization (e.g., `registry.k8s.io/coredns/coredns` → `coredns`).
- NVD fallback uses keyword search **without version filtering** — results may include false positives (logged as `matched_by=nvd-fallback`, `confidence=low`).

### Debugging Commands

```bash
# Agent: SBOM extraction logs
kubectl logs -n fortuna -l app=fortuna-agent --tail=500 | grep -iE 'sbom|extract|queue|fail'

# Core: SBOM ingest and CVE matching
kubectl logs -n fortuna -l app=fortuna-core --tail=500 | grep -iE '\[SBOM\]|\[CVE\]|matcher|fail'

# DB: recent SBOMs
kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT pod_name, namespace, image_name, package_count, sbom_source, confidence, created_at
   FROM sboms WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT 20;"

# DB: components for a specific SBOM
kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT component_name, component_version, purl, source
   FROM sbom_components WHERE sbom_id = <id> AND deleted_at IS NULL;"

# DB: CVE matches for a specific SBOM
kubectl exec -n fortuna <postgres-pod> -- psql -U postgres -d fortuna -c \
  "SELECT package_name, cve_id, severity, matched_by
   FROM cve_matches WHERE sbom_id = <id> AND deleted_at IS NULL;"
```

---

## Testing

```bash
# Unit tests — extractor and parsers
cd agent && go test ./pkg/sbom/extractor/ -v

# Distroless-specific tests
cd agent && go test ./pkg/sbom/extractor/ -v -run 'TestDistrolessParser|TestGoBinaryParser'

# Core SBOM tests
cd core && go test ./pkg/cve/matcher/ -v
```

---

## Related Components

- **CVE Scanner** — uses SBOMs for vulnerability matching
- **Risk Engine** — creates insights from SBOM + CVE matches
- **Core** — orchestrates SBOM ingestion and event publishing
- **Dashboard** — displays SBOM details, components, and CVE findings
