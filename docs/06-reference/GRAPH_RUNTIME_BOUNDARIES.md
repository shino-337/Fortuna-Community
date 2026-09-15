# Graph and runtime boundaries

The main graph uses the relational builder with an authorized cluster selection.
The cluster, clusterId and cluster_id aliases must agree. A scoped user with more
than one cluster must select one. Database/build errors are returned instead of
an empty fallback graph. Existing scoped attack-path summary, chains, objectives,
and bundle APIs use the same cluster resolver.

Legacy AGE blast-radius, shortest-path, accessible-resource, arbitrary Cypher,
permissions and risky-pods operations do not constrain every traversed node and
edge. They now require unrestricted cluster scope and reject cluster filters.
This closes scoped-data exposure; enabling scoped AGE traversal remains a feature
that requires an end-to-end scoped query implementation. The relational graph
and attack-path views remain the supported scoped alternatives. Missing engines,
query failures and malformed rows no longer become empty permission/risky-pod
success responses. The accessible-resource fallback explicitly reports 503.

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

Regression tests cover legacy graph guards, runtime aliases, cache mutation and
key collisions, and service-cache separation across clusters/credential changes.
Live AGE, PostgreSQL, service discovery and load validation remain lab gates.
