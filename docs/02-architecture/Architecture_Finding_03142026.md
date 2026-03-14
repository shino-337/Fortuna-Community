inding #1: Core isn’t clustered-aware for stateful workers and WebSocket hubs
Description
The Core deployment is stateless only to the extent that API traffic is handled by replicas; internal deduplication, WebSocket hubs (/api/v1/ws/*), and the in-memory active maps used by risk, insights, and Real-Time updates remain per process. The architecture docs (docs/02-architecture/COMPONENTS.md) call out shared PostgreSQL/NATS but no Redis/cache layer or leader election, and the DEPLOYMENT_AND_ARCHITECTURE_FAQ confirms Core replicas don’t coordinate those in-memory structures.

Impact
When a Core replica fails or scales horizontally, session affinity breaks, duplicated SBOM events arrive, and WebSocket clients connected to the failed pod permanently lose updates until they reconnect. There’s no failover plan for long-lived sockets or job deduplication.

Risk Level
High

Recommendation
Introduce a shared coordination layer (Redis, leader election via etcd/NATS, or Postgres advisory locks) for deduplication and session state; move WebSocket hubs behind a pub/sub broker (NATS or Redis Streams) so any replica can serve clients transparently. Document the elasticity strategy and ensure load balancer sticky sessions are optional, not required for correctness.

Finding #2: Agent SBOM ingestion queue keying and image parsing omit unique identifiers
Description
agent/internal/sbom/queue.go keys active/retryCount maps by namespace/name, and processor.convertToProto uses parseImageRef that only splits on the last colon, ignoring ports (registry:5000) and digest references. When pods recycle rapidly, the new pod’s UID differs but name stays the same, so the queue quietly skips SBOM extraction until the prior entry expires. Image metadata stored in Core (ImageTag, ImageName) is also misparsed for digests or port-qualified registries, undermining deduplication and insight accuracy.

Impact
Pods in high-churn clusters (deployments, jobs) can miss SBOM submissions entirely, leaving Core blind to their vulnerabilities. Misparsed image identities make CVE fingerprinting unreliable and can misattribute data across registries.

Risk Level
High

Recommendation
Key work-queue bookkeeping by immutable pod UID (and optional creation timestamp) instead of name, so each new workload is processed immediately. Replace parseImageRef with a registry-aware parser that handles registry:port/image and digest refs (@sha256:), ideally by reusing existing containerd/OCI parsing libraries or docker/distribution/reference. Ensure Core can differentiate digest-based releases from tags.

Finding #3: Full sync deletes critical resources when payloads are missing
Description
During agent sync (core/internal/service/agent_service.go, processSyncedServiceAccounts), the absence of serviceAccounts in a full sync automatically deletes every SA in the cluster row (data["serviceAccounts"] empty → delete all). The DEPLOYMENT_AND_ARCHITECTURE_FAQ confirms agents auto-populate cluster_id and the Core service cleans up stale data by default. If an agent temporarily fails to collect service accounts (e.g., RBAC restrictions or API throttling), the core instead wipes the DB and logs.

Impact
Transient collection failures (API throttling, permission errors, network hiccups) trigger mass deletions, corrupting dashboard state, destroying audit trails, and forcing operators to re-sync historical state manually.

Risk Level
Critical

Recommendation
Don’t auto-delete when a payload is missing; require an explicit “clear” flag or per-resource delete marker. If you must delete, queue a background reconciliation job that softly deletes only records still absent after multiple consistent empty reports. Log and alert on missing payload fields instead of acting immediately.

Finding #4: mTLS/secret rotation is fully manual and brittle
Description
All mutually authenticated channels (Agent ↔ Core, webhook, dashboard auth) depend on a single manual scripts/utils/create_mtls_secret.sh run, storing certs in secrets (fortuna-core-tls, fortuna-agent-tls, fortuna-webhook-tls). Rotation requires rerunning the script and restarting services, with no automation or operator guidance beyond the README sections referenced in DEPLOYMENT_AND_ARCHITECTURE_FAQ. There is no integration with cert-manager or Kubernetes native secret renewal.

Impact
Cert expiration or compromised keys mean downtime while operators regenerate keys, patch secrets, and restart pods. The manual process is error-prone (copy/paste failing), hard to audit, and incompatible with automated rotation policies expected in enterprise security programs.

Risk Level
Medium

Recommendation
Adopt Kubernetes-native certificate issuance (cert-manager, Vault PKI, or ACME) to manage the CA/leaf cert lifecycle. At minimum automate rotation via a dedicated workflow: generate new certs into new secrets, implement a rollout that swaps secrets and restarts pods with zero downtime, and document the steps/tasks in deploy/README.md. Support auto-rollover by refreshing mounted secrets without requiring full redeploy.

Finding #5: Observability lacks distributed tracing and centralized audit context
Description
Logging/metrics are limited to component-level stdout (agent logs, Core Gin logs, cleanup goroutines) plus Prometheus metrics (/metrics) and WebSocket updates. There’s no tracing (OpenTelemetry/SkyWalking) even though NATS introduces asynchronous flows (SBOM → Core → insights). Audit logs are created per action but scatter across tables with no trace identifiers connecting an Agent payload to resulting insights.

Impact
In production, root-causing delays (e.g., slow CVE matching, NATS backlog) is brittle; you can’t correlate Agent ingestion, Core worker status, and dashboard responses across components. Security event tracking requires stitching disparate logs manually, hindering incident response.

Risk Level
Medium

Recommendation
Introduce correlation IDs (e.g., attach X-Request-ID from Agent to Core threads, propagate to NATS messages) and wire them into logs/metrics. Adopt a tracing layer (OpenTelemetry/Gin middleware) to span Agent gRPC, Core workers, and WebSocket push paths. Aggregate audit events with these IDs so security teams can trace “Agent X triggered insight Y.” Document the observability expectations in ops docs.

Finding #6: Multi-cluster deployment lacks isolation and tenant-aware resource limits
Description
The FAQ confirms Fortuna runs a single Core controlling multiple clusters, but all clusters share one PostgreSQL/NATS and control plane resources. There’s no namespace/customer isolation, quota awareness, or “per-cluster ingestion limit,” so a noisy cluster can overwhelm the only DB or queue, and agents share the same JWT/auth credentials (from deploy/fortuna-core-deployment.yaml).

Impact
Without per-cluster rate limiting or resource partitioning, a rogue or bursty cluster can spike SBOM submission, create thousands of insights, and saturate PostgreSQL/NATS, impacting other tenants. Shared secrets increase blast radius if an agent is compromised.

Risk Level
High

Recommendation
Add per-cluster rate limiting (Core ingest layer should tag messages with cluster ID and reject/queue if rates exceed configurable thresholds). Introduce RBAC/secret isolation per cluster (e.g., issue cluster-specific agent TLS credentials). Consider multi-tenant partitioning in PostgreSQL (sharding by cluster ID) or at least enforce resource quotas via NATS JetStream consumers and Postgres statement timeouts so a single cluster can’t degrade the whole system. Outline these policies in architecture docs for clarity.

Finding #7: Deployment docs still expect manual sequencing despite helper scripts
Description
deploy/README.md lists a long manual order (CNI → storage → infra → mTLS → RBAC → label → Core/Agent/Dashboard), and while scripts/deploy/deploy-fortuna-robust.sh encapsulates it, Helm charts and documentation still expect operators to manually run infra + certs before Helm.

Impact
Manual steps increase operator time and error rates; Helm upgrades can be inconsistent without automation, especially during recovery scenarios. It also means there’s no guaranteed reproducible order for production.

Risk Level
Low

Recommendation
Codify the recommended sequence in a reusable tool (Helm hooks, an Argo CD PreSync job, or a Makefile target) so all deployments follow the same steps. Reference the script in the Helm README and add explicit “prerequisites” that fail fast if infra/certs are missing.