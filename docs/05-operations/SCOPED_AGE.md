# Cluster-scoped Apache AGE

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

There is no automatic import from the old shared `fortuna_graph`: legacy data
must be rebuilt from cluster-qualified evidence. QueryService exposes scoped
UID path lookup. Risk scores, runtime-evidence constraints and permissions have
no validated AGE contract, so these projections explicitly direct callers to
the existing relational risk/attack-path/RBAC APIs. Retired legacy HTTP AGE
routes remain unavailable; no restricted-user endpoint is reopened by this
internal change.

The permanent PostgreSQL workflow now uses the digest-pinned Apache AGE 1.6.0 /
PostgreSQL 16 image and executes `TestAGEScopedTraversalPostgres` in addition to
the populated migration/concurrency tests. It verifies duplicate resource names
and UIDs, foreign node/edge import, parameter data containing Cypher delimiters,
and unsupported constraints. Local checks use the same image.
