# Attack graph

What the attack graph does and does not claim, and how the internal Apache AGE graph is scoped per cluster.

## Graph and runtime boundaries

The main graph uses the relational builder with an authorized cluster selection.
The cluster, clusterId and cluster_id aliases must agree. A scoped user with more
than one cluster must select one. Database/build errors are returned instead of
an empty fallback graph. Existing scoped attack-path summary, chains, objectives,
and bundle APIs use the same cluster resolver.

The older AGE HTTP operations (blast-radius, shortest-path, accessible-resource,
arbitrary Cypher, permissions and risky-pods) no longer exist; use the relational
graph and scoped attack-path views instead. The cluster-scoped internal AGE
engine is not exposed through any arbitrary-query endpoint.

Runtime event ingest uses only `POST /api/v2/runtime/events`. File, Falco and eBPF
producers send the canonical v2 metadata, including `observed_at`, and retain
failed batches for retry. Runtime v1 read APIs remain. Configure scoped Agent
credentials for per-Agent, per-cluster authentication.

Attack-path cache keys distinguish global builds from a cluster literally named
__all_clusters__. Cached and singleflight results are copied before returning to
callers, so response-specific edits do not alter subsequent responses. The cache
still uses its existing short TTL; it is not a real-time security decision source.

Network service names are looked up using the requested cluster's stored
kubeconfig, never Core's own in-cluster credentials. Cache keys include cluster
and credential digest, entries expire after 30 seconds, failures do not reuse
expired results, and live lookup has a five-second deadline. Missing credentials
leave optional service names absent; observed connections remain visible.
Runtime signal list and count queries share the same principal and alias validation.

Regression tests verify retired routes return 404 and are absent from the router,
supported routes remain present, senders never downgrade, runtime aliases, cache mutation and
key collisions, and service-cache separation across clusters/credential changes.
PostgreSQL regressions run in CI.


Unused HTTP aliases are also retired:

| Removed route | Supported replacement |
| --- | --- |
| ServiceAccount bulk delete, bulk disable and disable-inactive | DELETE /api/v1/inventory/serviceaccounts/:uid, or a reviewed ServiceAccount mutation |
| GET /api/v1/cluster/info, GET /api/v1/cluster/:id/nodes | GET /api/v1/inventory/clusters, GET /api/v1/inventory/clusters/:id/nodes/:nodeName |
| GET /api/v1/risk/runtime/summary, GET /api/v1/risk/runtime/top | GET /api/v1/runtime/signals |
| GET /api/v2/runtime/pods/:uid/security-state | GET /api/v2/runtime/pods/:uid/facts |
| GET /api/v1/rbac/permission-catalog | GET /api/v1/governance/permission-explorer |
| GET /api/v1/malware/check, GET /api/v1/malware/stats, POST /api/v1/malware/db/upload | GET /api/v1/malware/threats; the malware feed is synced by Core |
| GET /api/v1/metrics/policy-evaluation-cost, GET /api/v1/debug/technique-overlay, GET /api/v1/governance/emergency-access, POST /api/v1/internal/trigger-cve-match | none |
| GET /api/v1/monitoring/agents | GET /api/v1/agents/status |
| /api/v1/policy/rules/:id (detail/update/delete/test/metrics/matches) | /api/v1/policy/rules/uid/:uid with the same operation |

Unused numeric-ID Pod YAML and ServiceAccount handlers, duplicate network and
insight-summary handlers, and the unregistered Prometheus query placeholder are
deleted. ServiceAccount scope and mutation regressions exercise the registered
inventory APIs. The route security inventory must match the actual router exactly.
The static retired-route check covers Core handlers, Agent, Dashboard, deployment
manifests and executable E2E scripts; mutation probes verify the check itself.

## Cluster-scoped Apache AGE

The internal AGE API requires a canonical, already-authorized cluster ID at
construction. There is no default/global cluster and no arbitrary Cypher or raw
SQL escape hatch. A SHA-256-derived graph name isolates each cluster physically;
all created vertices and edges also carry `cluster_id`. Writers reject foreign
properties and malformed labels/IDs. An explicit `Initialize` creates a graph;
unavailable extensions, query failures and unsupported evidence/risk projections
return errors instead of successful empty results.

All values use the AGE prepared parameter map on one pinned SQL connection.
See the [AGE query contract](https://age.apache.org/age-manual/master/intro/cypher.html).
Labels/property keys are validated identifiers, graph names are generated, and
traversal depth is bounded to 1–8. Entry/target vertices and traversed edges are
filtered by cluster. Every decoded intermediate vertex and edge must match the
selected cluster, with consistent edge endpoints, before any result is returned.
Foreign imported intermediate data fails the whole request. Type annotations
are removed outside JSON strings only; property text remains intact.

There is no automatic import from the older shared `fortuna_graph`; graph data
is rebuilt from cluster-qualified evidence. QueryService exposes scoped
UID path lookup. Risk scores, runtime-evidence constraints and permissions have
no validated AGE contract, so these projections explicitly direct callers to
the relational risk, attack-path and RBAC APIs.

The PostgreSQL CI job uses the digest-pinned Apache AGE 1.6.0 /
PostgreSQL 16 image and executes `TestAGEScopedTraversalPostgres` in addition to
the populated migration/concurrency tests. It verifies duplicate resource names
and UIDs, foreign node/edge import, parameter data containing Cypher delimiters,
and unsupported constraints. Local checks use the same image.
