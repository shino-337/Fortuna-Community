# Configuration reference

Core and the Agent are configured only through environment variables. The bundled values are set in `deploy/fortuna-core-deployment.yaml` and `deploy/fortuna-agent-daemonset.yaml`. Those manifests are generated from the Helm chart in `deploy/helm/fortuna`: change `deploy/helm/fortuna/values.yaml`, and use `core.extraEnv` or `agent.extraEnv` for any variable that has no chart value. A CI check (`scripts/verify/check-docs-sync.py`) fails when a manifest sets a variable that is not listed here.

Defaults below are the values in the code. "none" means the feature is off or the value is empty when the variable is unset. Boolean variables accept `true` unless the purpose says otherwise; several also accept `1`, `yes` or `on`.

## Core

### Server, database and messaging

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://postgres:postgres@postgres:5432/fortuna?sslmode=disable` | PostgreSQL connection string; Core retries the connection and exits if it never succeeds or migrations fail. |
| `NATS_ENDPOINT` | `nats://nats.fortuna.svc.cluster.local:4222` | NATS JetStream URL; if unreachable Core still starts, without the SBOM, CVE and risk pipeline. |
| `FORTUNA_NATS_STREAM_REPLICAS` | `3` | JetStream replicas for the streams Core creates (1 to 5); must not exceed the number of NATS servers. The Helm chart sets it to `min(3, nats.replicas)`. |
| `HTTP_PORT` | `8080` | Port of the REST API and dashboard backend. |
| `GRPC_PORT` | `9090` | Port of the gRPC AgentService used by Agents. |
| `LOG_LEVEL` | `info` | Printed at startup only; it does not change log verbosity. |
| `GIN_MODE` | none | `debug` keeps the HTTP framework in debug mode; any other value runs it in release mode. |
| `KUBECONFIG` | none | Kubeconfig used for Core's own Kubernetes API calls when `~/.kube/config` is absent; otherwise the in-cluster config is used. |
| `FORTUNA_JS_DURABLES` | `false` | `true` makes the SBOM and CVE JetStream consumers durable instead of ephemeral. |

### Authentication

| Variable | Default | Purpose |
| --- | --- | --- |
| `AUTH_ENABLED` | `true` | Must stay `true`. `false` gives every caller a synthetic admin and is accepted only with `FORTUNA_DEV_MODE=1`; otherwise Core does not start. |
| `FORTUNA_DEV_MODE` | none | Local development only (`1`, `true` or `yes`): allows `AUTH_ENABLED=false`, a JWT secret shorter than 32 bytes, and an ephemeral random JWT secret when none is set. |
| `JWT_SECRET` | none | Secret that signs login tokens. Required (or `FORTUNA_JWT_SECRET`) and at least 32 bytes; otherwise Core does not start. |
| `FORTUNA_JWT_SECRET` | none | Alternative name for the JWT secret, used only when `JWT_SECRET` is empty. |
| `TOKEN_EXPIRATION_HOURS` | `24` | Lifetime of login tokens in hours. |
| `FORTUNA_ALLOWED_ORIGINS` | none | Comma-separated extra CORS origins; `http://localhost:*` and `http://127.0.0.1:*` are always allowed. |
| `FORTUNA_WS_ALLOWED_ORIGINS` | none | Comma-separated origins allowed to open WebSockets; requests without an Origin and localhost origins are always allowed. |
| `FORTUNA_TRUSTED_PROXIES` | none | Comma-separated proxy IPs or CIDRs whose `X-Forwarded-For` / `X-Real-IP` are trusted for the client IP; an invalid entry stops Core. |
| `FORTUNA_SIEM_AUDIT_WEBHOOK` | none | URL that receives each security audit event as a JSON POST, in addition to the database record. |

### Bootstrap admin and migrations

These are read by the database migrations that run at Core startup.

| Variable | Default | Purpose |
| --- | --- | --- |
| `FORTUNA_ADMIN_USERNAME` | `admin` | Username of the bootstrap admin account. |
| `FORTUNA_ADMIN_PASSWORD` | none | Bootstrap admin password; when unset the built-in first-login password is used and a password change is forced. |
| `FORTUNA_DEFAULT_ADMIN_PASSWORD` | built-in first-login password | Overrides the built-in first-login password; an admin password equal to it is treated as a bootstrap credential. |
| `FORTUNA_BOOTSTRAP_DEFAULT_CREDENTIAL` | none | Marks the admin password as a first-login credential: it is not synced over an existing account and a change is forced at login. |
| `FORTUNA_FORCE_ADMIN_PASSWORD_CHANGE` | none | Forces the bootstrap admin to change the password at next login. |
| `FORTUNA_ADMIN_EMAIL` | `<username>@fortuna.local` | Email of the bootstrap admin account. |
| `FORTUNA_RISK_EVALUATOR_USERNAME` | `fortuna-risk-evaluator` | Username of the service account the risk evaluation CronJob signs in as. |
| `FORTUNA_RISK_EVALUATOR_PASSWORD` | none | Password of that service account (`risk-evaluator-password` in `fortuna-secrets`). When set, Core creates or updates the account with only `auth.session` and `risk.evaluate`; it never takes over an existing account of another role. Its role and scope cannot be changed or the account deleted through the API, only disabled. |
| `FORTUNA_ALLOW_WEAK_BOOTSTRAP_PASSWORD` | `false` | `true` creates the admin even if the password fails the password policy; otherwise the admin is not created and Core logs a warning. |
| `FORTUNA_ENABLE_SEED_DATA` | none | Seeds the capability metadata and promotion rules that the Capability Catalog and PCE need. |
| `ENVIRONMENT` | `development` | `production` or `staging` makes the SQL files of some early migrations mandatory instead of falling back to automatic schema creation. |
| `CLUSTER_ID_TO_UPDATE` | none | With `CLUSTER_DISPLAY_NAME`, renames this cluster id once during migration 053. |
| `CLUSTER_DISPLAY_NAME` | none | New display name for `CLUSTER_ID_TO_UPDATE` in migration 053. |

### Agent ingest

| Variable | Default | Purpose |
| --- | --- | --- |
| `FORTUNA_INGEST_TOKEN` | none | Shared token required on HTTP Agent and runtime ingest routes; without it (and without a credential registry) those routes fail closed. |
| `FORTUNA_ALLOW_UNAUTHED_INGEST` | none | Local development only (`1` or `true`): accepts HTTP ingest without a token when `FORTUNA_INGEST_TOKEN` is empty. Requires `FORTUNA_DEV_MODE=1`; Core refuses to start otherwise. |
| `FORTUNA_AGENT_CREDENTIAL_REGISTRY` | none | Path to the scoped Agent credential registry; when set, HTTP ingest requires per-Agent credentials and the shared token is not accepted. See [Agent identity](AGENT_IDENTITY.md). |
| `FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY` | none | Path to the registry that binds gRPC client certificates to Agent identities; requires `TLS_ENABLED=true`, otherwise Core does not start. Unset, Core refuses all Agent gRPC writes, so no SBOMs are ingested. |
| `FORTUNA_SOURCE_HEALTH_REGISTRY` | none | Trust registry used to verify signed runtime source-health reports; without it those reports are rejected. |
| `RATE_LIMIT_PER_CLUSTER_ENABLED` | `true` | Rate-limits inventory sync and SBOM ingest per cluster id. |
| `RATE_LIMIT_SYNC_PER_CLUSTER_RPS` | `10` | Sync requests per second allowed per cluster. |
| `RATE_LIMIT_SYNC_PER_CLUSTER_BURST` | `20` | Sync burst size per cluster. |
| `RATE_LIMIT_SBOM_PER_CLUSTER_RPS` | `50` | SBOM ingest requests per second allowed per cluster. |
| `RATE_LIMIT_SBOM_PER_CLUSTER_BURST` | `100` | SBOM ingest burst size per cluster. |
| `STALE_POD_CUTOFF_MINUTES` | `30` | Pods not updated by a sync within this many minutes are removed as stale. |
| `DEFAULT_CLUSTER_ID` | none | Cluster id used for admission requests and insights when an event carries none (otherwise `unknown`). Admission webhook violations are recorded only when it is set, because an admission request does not name its cluster. |

### TLS

| Variable | Default | Purpose |
| --- | --- | --- |
| `TLS_ENABLED` | `false` | `true` serves gRPC with mTLS using the three files below; a certificate that cannot be loaded stops Core. |
| `TLS_CERT_PATH` | `/etc/fortuna/tls/server/tls.crt` | Server certificate for gRPC. |
| `TLS_KEY_PATH` | `/etc/fortuna/tls/server/tls.key` | Server private key for gRPC. |
| `TLS_CA_CERT_PATH` | `/etc/fortuna/tls/server/ca.crt` | CA that client (Agent) certificates must chain to. |

### Admission webhook

| Variable | Default | Purpose |
| --- | --- | --- |
| `WEBHOOK_TLS_CERT_PATH` | `/etc/webhook/certs/tls.crt` | Certificate of the admission webhook HTTPS server on port 8443. |
| `WEBHOOK_TLS_KEY_PATH` | `/etc/webhook/certs/tls.key` | Private key of the admission webhook server. |
| `FORTUNA_WEBHOOK_FAIL_OPEN` | none | `1` or `true` admits a request when the webhook cannot parse or evaluate it; by default such requests are denied. |
| `ADMISSION_RISK_GATE_ENABLED` | enabled | `false` turns off the risk-score gate; any other value leaves it on. |
| `ADMISSION_RISK_SENSITIVE_NAMESPACES` | none | Comma-separated namespaces where the risk gate applies; with none listed the gate never triggers. |
| `ADMISSION_RISK_BLOCK_THRESHOLD` | `70` | The gate triggers when the highest risk score in the namespace reaches this value. |
| `ADMISSION_RISK_GATE_MODE` | `enforce` | `enforce` denies when the gate triggers; `audit` only logs. Unknown values mean `enforce`. |

### Pod Detail

| Variable | Default | Purpose |
| --- | --- | --- |
| `POD_DETAIL_ENCRYPTION_KEY` | none | Base64 of 32 bytes; encrypts process command lines, binary paths and working directories at rest. Without it Core warns and stores plaintext; an invalid value stops Core. |
| `POD_DETAIL_ENCRYPTION_KEY_PREVIOUS` | none | Comma-separated older keys, used only to decrypt rows written before a rotation. Setting it without a current key, or an invalid entry, stops Core. |
| `POD_DETAIL_NET_SPIKE_WINDOW_MINUTES` | `30` | Baseline window for the network queue spike detector. |
| `POD_DETAIL_NET_SPIKE_MIN_SAMPLES` | `5` | Baseline samples a connection needs before a spike can be reported. |
| `POD_DETAIL_NET_SPIKE_MULTIPLIER` | `4.0` | A spike is reported when the current queue is at least this multiple of the baseline average. |
| `POD_DETAIL_NET_SPIKE_MIN_QUEUE_BYTES` | `4096` | Queue sizes below this are never reported as spikes. |
| `POD_DETAIL_NET_SPIKE_COOLDOWN_MINUTES` | `10` | Suppresses repeat spike events for the same connection within this window. |

### Risk engine

| Variable | Default | Purpose |
| --- | --- | --- |
| `FORTUNA_RULES_DIR` | none (falls back to `./rules` or `core/rules` if present) | Directory of YAML risk rules; also where rules edited in the UI are exported (`<dir>/risk`). |
| `FORTUNA_RISK_CONTEXT_TIER` | `staging` | Environment tier for the risk context multiplier: `dev` (0.9), `staging` (1.0) or `prod` (1.15). |
| `FORTUNA_RUNTIME_RISK_LOOKBACK_HOURS` | `24` | Hours of runtime signals considered when enriching pods for rule evaluation (1 to 168). |
| `FORTUNA_SECURITY_STATE_CACHE_TTL` | `5m` | Minimum age before a pod's security state is recomputed; values above 5m are ignored. |
| `FORTUNA_TECHNIQUE_SOURCE` | none | `go` turns off MITRE, risk and tactic semantics from the technique overlay YAML. |
| `FORTUNA_MITRE_ENFORCE_SCOPE` | none | `1` matches node-scoped MITRE steps only to runtime events that carry a node name (stricter, less coverage). |
| `FORTUNA_UI_RISK_LEGACY_ROOT_FIELDS` | none | Adds a `legacy` object (highest severity, priority level) to each risk list row in the API. |
| `RUNTIME_ATTACK_RESCORE_ENABLED` | `true` | Re-evaluates runtime rules and risk scores for a pod after attack-like Falco events. |
| `RUNTIME_ATTACK_RESCORE_DEBOUNCE` | `30s` | Waits this long after the latest event before rescoring a pod. |
| `RUNTIME_ATTACK_RESCORE_MIN_SEVERITY` | `medium` | Lowest event severity that triggers a rescore when the event has no named Falco rule. |
| `RUNTIME_ATTACK_RESCORE_MAX_BATCH_PODS` | `50` | Read at startup but not currently used. |
| `RISK_SCORE_V3_BACKFILL_ENABLED` | `true` | Runs the periodic job that recomputes V3 risk scores for all pods and resources with open insights. |
| `RISK_SCORE_V3_BACKFILL_INTERVAL` | `6h` | Interval of the V3 risk score backfill job. |
| `RISK_SCORE_V3_BACKFILL_ITEM_DELAY` | `0` | Pause between resources in the backfill job, to spread database load. |

### Runtime signal mappings

| Variable | Default | Purpose |
| --- | --- | --- |
| `FORTUNA_RUNTIME_MAPPING_WRITE_MODE` | `ENABLED` | Writes to runtime signal-to-step mappings: `ENABLED`, `DRY_RUN` or `DISABLED` (writes rejected). |
| `FORTUNA_RUNTIME_MAPPING_STRICT_SIGNAL_UNIQUE` | `false` | `true` rejects mapping a signal type that is already mapped to another step. |
| `FORTUNA_RUNTIME_MAPPING_BLAST_RADIUS_ABS_CAP` | `10000` | Upper limit on rows a mapping change may affect without `force=true`. |
| `FORTUNA_RUNTIME_MAPPING_BLAST_RADIUS_RATIO` | `0.2` | Limit as a share of the last 24 hours of runtime signals; the lower of the two limits applies, never below 50. |
| `FORTUNA_RUNTIME_MAPPING_AUDIT_ANCHOR_EVERY` | `100` | Writes an audit anchor hash every this many mapping audit records. |

### Attack paths

| Variable | Default | Purpose |
| --- | --- | --- |
| `ATTACK_PATH_RECONCILE_INTERVAL` | `30m` | Interval of the job that recomputes attack paths for all pods. |
| `FORTUNA_ATTACK_PATH_BUILD_CACHE_TTL` | `45s` | How long a cluster's built attack paths are cached; `0` disables the cache. |
| `FORTUNA_ATTACK_PATH_OBSERVED_EGRESS_WINDOW` | `24h` | Observed egress to a pod IP within this window counts as network reachability. |
| `FORTUNA_ATTACK_PATH_FALLBACK_TTL` | `6h` | New pods with no restarts younger than this get soft reachability when there is no other evidence; `0` disables. |
| `FORTUNA_ATTACK_PATH_DENY_TTL` | `15m` | Network policy deny signals within this window mark an edge unreachable. |
| `FORTUNA_ATTACK_PATH_HYSTERESIS_GRACE` | `60m` | Paths that disappear are kept, with decayed scores, for this long before deletion; `0` deletes them at once. |
| `FORTUNA_ATTACK_PATH_HYSTERESIS_DECAY` | `0.7` | Factor (0 to 1) applied to the risk and impact of a kept path on each recomputation. |

### SBOM and vulnerabilities

| Variable | Default | Purpose |
| --- | --- | --- |
| `FORTUNA_SBOM_ORPHAN_GRACE_PERIOD` | `30m` | An SBOM whose pod is missing is kept for at least this long (duration or minutes). |
| `FORTUNA_SBOM_DLQ_DEPTH_POLL_INTERVAL` | `30s` | How often the SBOM dead-letter queue depth is polled for metrics; `0` or `off` disables. |
| `FORTUNA_SBOM_DLQ_REPLAY_MAX_ATTEMPTS` | `5` | Replay attempts for a dead-lettered SBOM event before it is dropped. |
| `FORTUNA_OSV_SOURCE_DIR` | none | Directory of OSV JSON files loaded into the vulnerability mirror at startup when it is empty, and on database updates. |
| `FORTUNA_CVE_CACHE_MAX_ENTRIES` | `10000` | Maximum entries in the in-memory CVE query cache. |
| `FORTUNA_K8S_COMPONENT_MAP_PATH` | none | Path to the Kubernetes component to module mapping YAML; checked before the built-in locations. |
| `FORTUNA_KEV_ENABLED` | none | Refreshes the CISA Known Exploited Vulnerabilities catalog and flags matching CVEs. |
| `FORTUNA_KEV_URL` | CISA KEV JSON feed | Alternative URL for the KEV feed. |
| `FORTUNA_KEV_REFRESH` | `6h` | KEV catalog refresh interval. |
| `FORTUNA_EPSS_ENABLED` | none | Looks up FIRST EPSS scores for matched CVEs. |
| `FORTUNA_EPSS_BASE_URL` | `https://api.first.org/data/v1/epss` | Alternative EPSS API base URL. |
| `FORTUNA_EPSS_CACHE_TTL` | `24h` | How long EPSS scores are cached in memory. |
| `FORTUNA_EPSS_CONCURRENCY` | `8` | Concurrent EPSS API requests. |
| `FORTUNA_EPSS_MAX_PER_SBOM` | `40` | Maximum CVEs per SBOM looked up in EPSS; `0` disables, a negative value means 10000. |
| `FORTUNA_TRUSTED_REGISTRIES` | none | Comma-separated registries shown as trusted in image trust (`host`, `*.domain` or `prefix/*`). |
| `FORTUNA_BLOCKED_REGISTRIES` | none | Comma-separated registries shown as blocked in image trust; checked before the trusted list. |

### Scheduled jobs and retention

| Variable | Default | Purpose |
| --- | --- | --- |
| `PCE_SCHEDULER_ENABLED` | `true` | Runs the periodic Pod Capability Engine evaluation. |
| `PCE_SCHEDULER_INTERVAL` | `6h` | Interval of the PCE evaluation; invalid values mean `6h`. |
| `PCE_CLEANUP_RETENTION_DAYS` | `30` | Deletes pod capabilities not seen for this many days (daily job). |
| `INSIGHTS_RESOLVED_RETENTION_DAYS` | `30` | Soft-deletes resolved insights older than this many days. |
| `INSIGHTS_ACTIVE_RETENTION_DAYS` | `90` | Soft-deletes active insights not updated for this many days. |
| `POD_NETWORK_RETENTION_HOURS` | `24` | Deletes Pod network connection rows older than this many hours. |
| `POD_NETWORK_CLEANUP_INTERVAL` | `10m` | Interval of the network connection cleanup. |
| `POD_NETWORK_CLEANUP_BATCH` | `20000` | Rows deleted per cleanup round. |
| `POD_NETWORK_CLEANUP_ROUNDS` | `3` | Maximum cleanup rounds per run. |
| `POD_NETWORK_CLEANUP_INITIAL_DELAY` | `0` | Delay before the first cleanup run after startup. |
| `POD_PROCESS_RETENTION_HOURS` | `24` | Deletes Pod process snapshots older than this many hours; the Pod detail view shows only the latest snapshot. |
| `POD_PROCESS_CLEANUP_INTERVAL` | `1h` | Interval of the process snapshot cleanup. |
| `FORTUNA_RETENTION_RUNTIME_DAYS` | `30` | Deletes runtime events, behavior facts, signals and incidents last seen more than this many days ago. `0` keeps them. |
| `FORTUNA_RETENTION_K8S_EVENTS_DAYS` | `14` | Deletes Kubernetes events older than this many days. `0` keeps them. |
| `FORTUNA_RETENTION_POD_METRICS_DAYS` | `30` | Deletes Pod runtime metrics not observed for this many days. `0` keeps them. |
| `FORTUNA_RETENTION_SESSIONS_DAYS` | `30` | Deletes login sessions that expired more than this many days ago. `0` keeps them. |
| `FORTUNA_RETENTION_AUDIT_LOG_DAYS` | `90` | Deletes `audit_logs` rows older than this many days. `0` keeps them. The append-only security activity log is never deleted. |
| `FORTUNA_RETENTION_CASES_DAYS` | `1` | Any value above `0` hard-deletes archived investigation cases once their `retention_until` date passes. `0` keeps them. |
| `FORTUNA_RETENTION_INTERVAL` | `6h` | Interval of the retention job. Each run also clears the password hash of deleted users. |

### Metrics and limits

| Variable | Default | Purpose |
| --- | --- | --- |
| `FORTUNA_METRICS_ADDR` | none | Listen address (for example `:9091`) of a separate, unauthenticated Prometheus `/metrics` server; unset disables it. |
| `FORTUNA_MAX_REQUEST_BODY_BYTES` | `67108864` (64 MiB) | Maximum HTTP request body size. |
| `RISKS_WS_MAX_CONNS_PER_IP` | `10` | Maximum risk WebSocket connections per client IP. |
| `NETWORK_ACTIVITY_MAX_PAGE_SIZE` | `200` | Page size cap for network activity list views (at most 500). |
| `NETWORK_ACTIVITY_TOPOLOGY_EDGES_MAX` | `2500` | Page size cap for the network topology edges view (at most 8000). |
| `PIPELINE_HEALTH_LAYER1_HEALTHY_MINUTES` | `30` | Pipeline health: PCE evaluation is healthy if newer than this. |
| `PIPELINE_HEALTH_LAYER1_DEGRADED_MINUTES` | `180` | Pipeline health: PCE evaluation is degraded, not stale, if newer than this. |
| `PIPELINE_HEALTH_LAYER2_HEALTHY_MINUTES` | `30` | Pipeline health: capability state change is healthy if newer than this. |
| `PIPELINE_HEALTH_LAYER2_DEGRADED_MINUTES` | `180` | Pipeline health: capability state change is degraded, not stale, if newer than this. |
| `PIPELINE_HEALTH_LAYER3_HEALTHY_MINUTES` | `30` | Pipeline health: attack path computation is healthy if newer than this. |
| `PIPELINE_HEALTH_LAYER3_DEGRADED_MINUTES` | `180` | Pipeline health: attack path computation is degraded, not stale, if newer than this. |
| `PIPELINE_HEALTH_LAYER4_HEALTHY_MINUTES` | `30` | Pipeline health: risk score calculation is healthy if newer than this. |
| `PIPELINE_HEALTH_LAYER4_DEGRADED_MINUTES` | `180` | Pipeline health: risk score calculation is degraded, not stale, if newer than this. |

Not listed: `FORTUNA_SKIP_ROUTE_SECURITY_VERIFY` and `FORTUNA_ROUTE_SECURITY_REPORT`, internal hooks for the route security check that is run in CI, and `FORTUNA_REHEARSAL_POSTGRES_URL`, read only by the `core/cmd/migration-rehearsal` tool.

## Agent

The Agent has no log level setting.

### Core connection

| Variable | Default | Purpose |
| --- | --- | --- |
| `CORE_GRPC_ENDPOINT` | `fortuna-core.fortuna.svc.cluster.local:9090` | Core gRPC address for registration, heartbeats and SBOM upload; the Agent retries until it connects. |
| `CORE_HTTP_ENDPOINT` | `http://fortuna-core.fortuna.svc.cluster.local:8080` | Core HTTP base URL for inventory sync, Pod Detail, events and runtime ingest. |
| `FORTUNA_INGEST_TOKEN` | none | Shared token sent on HTTP ingest requests; must match Core's `FORTUNA_INGEST_TOKEN`. |
| `FORTUNA_AGENT_TOKEN_FILE` | none | File holding a scoped Agent credential, reread on every request; when set it replaces the shared token on ingest routes and a missing or invalid file fails closed. |
| `FORTUNA_CORE_HTTP_AUTHORIZATION` | none | Value sent as the `Authorization` header on every Core HTTP request, for a reverse proxy in front of Core. |
| `SYNC_INTERVAL` | `30s` | Interval of the full inventory sync (pods, RBAC, resources) to Core. |
| `HEARTBEAT_INTERVAL` | `15s` | Interval of the gRPC ping that keeps the Agent's last-seen time fresh when Core has the gRPC registry configured (minimum 5s). |
| `BATCH_SIZE` | `50` | Read but not currently used. |
| `BATCH_TIMEOUT_MS` | `5000` | Read but not currently used. |

### Identity

| Variable | Default | Purpose |
| --- | --- | --- |
| `NODE_NAME` | `unknown-node` | Node the Agent runs on, set from `spec.nodeName`; without it SBOM and Pod Detail find no pods. |
| `NODE_IP` | value of `NODE_NAME` | Node id reported at registration and with SBOMs. |
| `AGENT_ID` | `<NODE_NAME>-agent` | Agent id reported to Core. |
| `CLUSTER_ID` | none (discovered) | Overrides the cluster id discovered from the Kubernetes API; discovery failure stops the Agent. |
| `CLUSTER_NAME` | value of `CLUSTER_ID` | Cluster display name; used only when `CLUSTER_ID` is set. |
| `POD_UID` | none | UID of the Agent pod, attached to eBPF sensor events. |
| `KUBECONFIG` | none | Kubeconfig path; unset uses the in-cluster service account. |

### Collection

| Variable | Default | Purpose |
| --- | --- | --- |
| `WATCH_NAMESPACE` | none (all namespaces) | Limits the inventory sync to one namespace. |
| `POD_DETAIL_REPORT_INTERVAL` | `2m` | Interval of Pod Detail reports (process and socket snapshots, metrics) to Core. |
| `POD_DETAIL_PROC_ROOT` | `/host/proc` | Path of the read-only host `/proc` mount used for processes and sockets. |
| `POD_DETAIL_PROCESS_CPU_APPROX_FROM_PROC` | `true` | Approximates per-process CPU from `/proc`; `false`, `0`, `no` or `off` disables. |
| `POD_DETAIL_RUNTIME_SOURCE` | none | Obsolete: `exec` (or `0`, `false`) only logs that exec collection was removed; data always comes from host `/proc`. |

### SBOM

| Variable | Default | Purpose |
| --- | --- | --- |
| `SBOM_WORKERS` | `2` | Concurrent SBOM extractions (1 to 4); lower values reduce peak memory. |
| `IMAGE_EXPORT_SOCKET` | none (Agent), `/run/fortuna-image-export/export.sock` (image-export) | Agent: fetch image archives from the image-export helper on this socket instead of containerd. Image-export: socket it listens on. |
| `IMAGE_EXPORT_MAX_BYTES` | `8589934592` (8 GiB) | Image-export container only: maximum archive size per image; an invalid value stops the helper. |
| `CONTAINERD_SOCKET` | `/run/containerd/containerd.sock` | Containerd socket, used by the image-export container, or by the Agent when `IMAGE_EXPORT_SOCKET` is unset. |
| `CONTAINERD_NAMESPACE` | `k8s.io` | Containerd namespace that holds Kubernetes images. |
| `SBOM_PREFER_REGISTRY` | none | `1` or `true` pulls images from the registry instead of trying containerd first. |
| `SBOM_CACHE_DIR` | `/var/lib/fortuna/sbom-cache` | On-disk SBOM cache keyed by image digest; `0`, `off` or `disabled` turns it off. |
| `SBOM_CACHE_MAX_AGE` | none | Cache entries older than this duration are removed. |
| `SBOM_CACHE_MAX_TOTAL_BYTES` | none | Oldest cache files are removed when the cache exceeds this size. |
| `SBOM_CACHE_MAX_FILES` | none | Oldest cache files are removed when the cache holds more than this many files. |
| `SBOM_FS_MODE` | `materialize` | `materialize` keeps extracted file contents in memory; `indexed` keeps a path index and reads contents lazily from spooled layers. |
| `SBOM_FS_SPOOL_DIR` | system temp directory (`TMPDIR`) | Where `indexed` mode spools uncompressed layers. |
| `SBOM_FS_MAX_TOTAL_BYTES` | `1073741824` (1 GiB) | Maximum file content kept in memory per image. |
| `SBOM_FS_MAX_FILE_BYTES` | `67108864` (64 MiB) | Files larger than this are not read. |
| `SBOM_FS_SKIP_PATH_PREFIXES` | none | Comma-separated absolute path prefixes whose files are not stored; `off` disables. |
| `SBOM_FS_METRICS` | on | `0`, `off` or `false` stops the per-image filesystem metrics log line. |
| `FORTUNA_RPM_INVENTORY_PATH` | `/var/lib/fortuna/rpm-packages.list` | Path inside the image of an RPM package list, tried before rpm manifests and the rpm database. |
| `SBOM_ENGINE` | `syft` | Main package cataloger. With `syft`, Syft catalogs the exported image offline and the Fortuna parsers are used only when Syft is missing or fails; `fortuna` uses the Fortuna parsers first. |
| `SBOM_USE_SYFT_FALLBACK` | enabled | `0`, `false`, `off` or `disabled` stops running Syft (when `SBOM_ENGINE=fortuna`) for distroless or unknown images where few packages were found. |
| `SBOM_SYFT_BIN` | `syft` | Syft executable. |
| `SBOM_SYFT_MIN_PACKAGE_THRESHOLD` | `20` | Distroless images with fewer packages than this also get a Syft pass. |
| `SBOM_SYFT_TIMEOUT` | `5m` | Timeout of one Syft run (duration or seconds). |
| `SBOM_SYFT_MAX_PACKAGES` | `10000` | Maximum packages taken from a Syft result. |
| `SBOM_SYFT_MAX_RETRIES` | `2` | Retries of a failed Syft run. |
| `SBOM_SYFT_CACHE_TTL` | `1h` | How long Syft results are cached in memory per digest; `0` disables. |
| `SBOM_SYFT_CACHE_MAX_ITEMS` | `256` | Maximum cached Syft results; `0` disables. |

### Runtime sensors

See [runtime evidence](RUNTIME_EVIDENCE.md) for what these sources prove.

| Variable | Default | Purpose |
| --- | --- | --- |
| `FALCO_EVENTS_ENABLED` | `false` | Reads Falco JSON output and sends it to Core. |
| `FALCO_EVENTS_PATH` | `/var/log/falco/events.jsonl` | Falco JSONL file to read. |
| `FALCO_EVENTS_POLL` | `5s` | Poll interval of the Falco file. |
| `FALCO_DELIVERY_STATE_PATH` | none | File for the durable Falco cursor and outbox, so delivery resumes after a restart; unset keeps no durable state. |
| `RUNTIME_EVENTS_ENABLED` | `false` | Reads a generic runtime events JSONL file and sends it to Core. |
| `RUNTIME_EVENTS_PATH` | `/var/log/fortuna/runtime-events.log` | Runtime events file to read. |
| `RUNTIME_EVENTS_POLL` | `5s` | Poll interval of the runtime events file. |
| `RUNTIME_COVERAGE_CADENCE` | `30s` | Window over which producer coverage counts are aggregated before reporting; failures are reported at once. |
| `RUNTIME_SOURCE_HEALTH_PATH` | none | File of sensor-signed source-health reports that the Agent relays to Core. |
| `RUNTIME_SOURCE_HEALTH_CHALLENGE_PATH` | none | File where the Agent writes its session challenge for a sensor to sign; a write failure stops the Agent. |
| `EBPF_ENABLED` | `false` | Starts the experimental eBPF sensor; the bundled manifest does not grant the capabilities it needs. |
| `EBPF_MODE` | `exec` | eBPF sensor mode: `exec`, `connect` or `all`. |
| `EBPF_EVENT_FLUSH_INTERVAL` | `5s` | How often buffered eBPF events are sent to Core. |
| `EBPF_EVENT_BUFFER_SIZE` | `200` | eBPF event buffer size. |
| `EBPF_SIMULATE` | `false` | Emits synthetic eBPF events for testing; never evidence of real behavior. |

### TLS

| Variable | Default | Purpose |
| --- | --- | --- |
| `TLS_ENABLED` | `true` | Connects to Core gRPC with mTLS using the three files below. |
| `TLS_CERT_PATH` | `/etc/fortuna/tls/client/tls.crt` | Agent client certificate. |
| `TLS_KEY_PATH` | `/etc/fortuna/tls/client/tls.key` | Agent client private key. |
| `TLS_CA_CERT_PATH` | `/etc/fortuna/tls/client/ca.crt` | CA used to verify Core's server certificate. |

## Process environment

The Agent manifest also sets these variables. Fortuna code does not read them; they make SBOM tooling such as Syft write to the scratch volume mounted at `/tmp`, because the root filesystem is read-only.

| Variable | Value in the manifest | Purpose |
| --- | --- | --- |
| `HOME` | `/tmp` | Home directory for tools that write configuration or state there. |
| `TMPDIR` | `/tmp` | Temporary files, including the default `SBOM_FS_SPOOL_DIR`. |
| `XDG_CACHE_HOME` | `/tmp/.cache` | Cache directory for tools that follow the XDG convention. |
