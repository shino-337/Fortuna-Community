# Graph and runtime boundaries

The main graph uses the relational builder with an authorized cluster selection.
The cluster, clusterId and cluster_id aliases must agree. A scoped user with more
than one cluster must select one. Database/build errors are returned instead of
an empty fallback graph. Existing scoped attack-path summary, chains, objectives,
and bundle APIs use the same cluster resolver.

The six legacy AGE HTTP operations (blast-radius, shortest-path,
accessible-resource, arbitrary Cypher, permissions and risky-pods) are removed,
including their route registrations and handlers. The relational graph and scoped
attack-path views remain supported. No in-repository client used the retired
graph endpoints; external callers must migrate to these views. The internal AGE
engine is not exposed through a replacement arbitrary-query endpoint.

Runtime event ingest uses only POST /api/v2/runtime/events. File, Falco and eBPF
producers send the canonical v2 metadata, including observed_at; they retain failed
batches for retry without downgrading to v1. Runtime v1 read APIs remain supported.
This is a coordinated Core/Agent upgrade: upgrade agents to this v2-capable build
before or alongside Core. Old agents that only send v1 cannot ingest after removal.
Shared-token authentication migration mode is unchanged; configure scoped agent
credentials for per-agent/per-cluster authentication.

Attack-path cache keys distinguish global builds from a cluster literally named
__all_clusters__. Cached and singleflight results are copied before returning to
callers, so response-specific edits do not alter subsequent responses. The cache
still uses its existing short TTL; it is not a real-time security decision source.

Network service names are looked up using the requested cluster's stored
kubeconfig, never Core's own in-cluster credentials. Cache keys include cluster
and credential digest, entries expire after 30 seconds, failures do not reuse
expired results, and live lookup has a five-second deadline. Missing credentials
leave optional service names absent; observed connections remain visible.
Runtime signal list/count queries now share typed-principal and alias validation.

Regression tests verify retired routes return 404 and are absent from the router,
supported routes remain present, senders never downgrade, runtime aliases, cache mutation and
key collisions, and service-cache separation across clusters/credential changes.
PostgreSQL regressions run in CI. Live service discovery, coordinated upgrades
and load validation remain lab gates.


Unused HTTP aliases are also retired:

| Removed route | Supported replacement |
| --- | --- |
| POST /api/v1/bulk/serviceaccounts/disable | POST /api/v1/inventory/serviceaccounts/bulk/disable |
| DELETE /api/v1/bulk/serviceaccounts/delete | POST /api/v1/inventory/serviceaccounts/bulk/delete |
| GET /api/v1/monitoring/agents | GET /api/v1/agents/status |
| /api/v1/policy/rules/:id (detail/update/delete/test/metrics/matches) | /api/v1/policy/rules/uid/:uid with the same operation |

Unused numeric-ID Pod YAML and ServiceAccount handlers, duplicate network and
insight-summary handlers, and the unregistered Prometheus query placeholder are
deleted. ServiceAccount scope and mutation regressions exercise the registered
inventory APIs. The route security inventory must match the actual router exactly.
The static retired-route check covers Core handlers, Agent, Dashboard, deployment
manifests and executable E2E scripts; mutation probes verify the check itself.
