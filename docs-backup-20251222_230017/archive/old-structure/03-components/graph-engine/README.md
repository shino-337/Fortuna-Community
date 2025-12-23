# Graph Engine Component

The Graph Engine uses Apache AGE (A Graph Extension) on PostgreSQL to build and query security relationship graphs for attack path analysis.

---

## Overview

**Status**: ✅ Production Ready (MVP1)
**Technology**: Apache AGE (PostgreSQL extension)
**Query Language**: Cypher (same as Neo4j)
**Use Cases**: Attack paths, blast radius, permission chains

---

## Architecture

```
Resource Data (ServiceAccounts, Pods, Roles, RoleBindings)
    ↓
Correlator Worker
    ├─▶ Create Vertices (nodes)
    │    ├─▶ ServiceAccount nodes
    │    ├─▶ Pod nodes
    │    ├─▶ Role nodes
    │    └─▶ Namespace nodes
    │
    └─▶ Create Edges (relationships)
         ├─▶ SA --USES--> Pod
         ├─▶ SA --HAS_ROLE--> Role
         ├─▶ Pod --IN_NAMESPACE--> Namespace
         └─▶ Pod --ACCESSES--> Service
    ↓
Graph Database (Apache AGE)
    ↓
Cypher Queries
    ├─▶ Attack Path Analysis
    ├─▶ Blast Radius Calculation
    ├─▶ Permission Chains
    └─▶ Lateral Movement Detection
```

---

## Graph Schema

### Vertex Types (Nodes)

```cypher
// ServiceAccount
(:ServiceAccount {
  uid: "abc-123",
  name: "default",
  namespace: "kube-system",
  risk_score: 7.5
})

// Pod
(:Pod {
  uid: "def-456",
  name: "nginx-prod",
  namespace: "production",
  image: "nginx:1.21.0"
})

// Role
(:Role {
  uid: "ghi-789",
  name: "pod-reader",
  type: "Role",  // or ClusterRole
  rules: [...]
})

// Namespace
(:Namespace {
  name: "production",
  sensitivity: "high"
})
```

### Edge Types (Relationships)

```cypher
// ServiceAccount uses Pod
(:ServiceAccount)-[:USES]->(:Pod)

// ServiceAccount has Role
(:ServiceAccount)-[:HAS_ROLE {
  binding_name: "admin-binding",
  binding_type: "ClusterRoleBinding"
}]->(:Role)

// Pod in Namespace
(:Pod)-[:IN_NAMESPACE]->(:Namespace)

// Pod mounts Secret
(:Pod)-[:MOUNTS_SECRET]->(:Secret)

// Role allows access to Resource
(:Role)-[:CAN_ACCESS {
  verbs: ["get", "list", "delete"],
  resources: ["pods"]
}]->(:ResourceType)
```

---

## Database Setup

### Enable Apache AGE Extension

```sql
-- Create AGE extension
CREATE EXTENSION IF NOT EXISTS age;

-- Load AGE extension
LOAD 'age';

-- Set search path
SET search_path = ag_catalog, "$user", public;
```

### Create Graph

```sql
-- Create KSAM graph
SELECT create_graph('ksam_security_graph');
```

---

## Query Examples

### 1. Find All Pods Using ServiceAccount

```cypher
SELECT * FROM cypher('ksam_security_graph', $$
  MATCH (sa:ServiceAccount {name: 'default', namespace: 'kube-system'})
        -[:USES]->(pod:Pod)
  RETURN sa.name, pod.name, pod.namespace
$$) as (sa_name agtype, pod_name agtype, pod_namespace agtype);
```

### 2. Find ServiceAccounts with Cluster-Admin

```cypher
SELECT * FROM cypher('ksam_security_graph', $$
  MATCH (sa:ServiceAccount)-[:HAS_ROLE]->(role:Role {name: 'cluster-admin'})
  RETURN sa.namespace, sa.name, role.type
$$) as (namespace agtype, sa_name agtype, role_type agtype);
```

### 3. Attack Path: ServiceAccount → Pod → Secret

```cypher
SELECT * FROM cypher('ksam_security_graph', $$
  MATCH path = (sa:ServiceAccount)-[:USES]->(pod:Pod)
                                  -[:MOUNTS_SECRET]->(secret:Secret)
  WHERE sa.namespace = 'default'
  RETURN sa.name,
         pod.name,
         secret.name,
         length(path) as path_length
$$) as (sa_name agtype, pod_name agtype, secret_name agtype, path_length agtype);
```

### 4. Blast Radius: All Resources Accessible by ServiceAccount

```cypher
SELECT * FROM cypher('ksam_security_graph', $$
  MATCH (sa:ServiceAccount {name: 'admin-sa'})-[:HAS_ROLE]->(role:Role)
        -[:CAN_ACCESS]->(resource:ResourceType)
  RETURN sa.name,
         role.name,
         resource.kind,
         resource.verbs
$$) as (sa_name agtype, role_name agtype, resource_kind agtype, verbs agtype);
```

### 5. Shortest Attack Path Between Two Resources

```cypher
SELECT * FROM cypher('ksam_security_graph', $$
  MATCH path = shortestPath(
    (start:ServiceAccount {name: 'attacker-sa'})
    -[*]->
    (target:Secret {name: 'db-credentials'})
  )
  RETURN nodes(path), relationships(path), length(path)
$$) as (nodes agtype, relationships agtype, path_length agtype);
```

---

## API Endpoints

### Graph Queries

```bash
# Get attack paths from ServiceAccount
curl http://localhost:8080/api/v1/graph/attack-paths \
  -d '{
    "source_type": "ServiceAccount",
    "source_namespace": "default",
    "source_name": "admin-sa",
    "target_type": "Secret"
  }'

# Response:
{
  "paths": [
    {
      "nodes": [
        {"type": "ServiceAccount", "name": "admin-sa"},
        {"type": "Pod", "name": "admin-pod"},
        {"type": "Secret", "name": "db-credentials"}
      ],
      "edges": [
        {"type": "USES"},
        {"type": "MOUNTS_SECRET"}
      ],
      "length": 2,
      "risk_score": 8.5
    }
  ]
}
```

### Blast Radius

```bash
# Get blast radius for resource
curl http://localhost:8080/api/v1/graph/blast-radius/{resource_type}/{namespace}/{name}

# Response:
{
  "source": {
    "type": "ServiceAccount",
    "namespace": "kube-system",
    "name": "default"
  },
  "accessible_resources": {
    "pods": 15,
    "secrets": 3,
    "configmaps": 8,
    "services": 5
  },
  "risk_level": "CRITICAL"
}
```

### Relationship Queries

```bash
# Get all relationships for resource
curl http://localhost:8080/api/v1/graph/relationships/{resource_type}/{namespace}/{name}

# Response:
{
  "resource": "ServiceAccount/kube-system/default",
  "relationships": {
    "uses_pods": ["pod-1", "pod-2"],
    "has_roles": ["cluster-admin"],
    "can_access": ["secrets", "configmaps", "pods"]
  }
}
```

---

## Correlator Worker

### Processing Flow

```go
// core/pkg/worker/correlator_worker.go

type CorrelatorWorker struct {
    db          *gorm.DB
    graphEngine *graph.AgeEngine
}

func (w *CorrelatorWorker) ProcessServiceAccount(ctx context.Context, sa *models.ServiceAccount) error {
    // 1. Create/update ServiceAccount vertex
    w.graphEngine.UpsertVertex("ServiceAccount", sa.UID, map[string]interface{}{
        "name":       sa.Name,
        "namespace":  sa.Namespace,
        "risk_score": sa.RiskScore,
    })

    // 2. Find all pods using this SA
    var pods []models.Pod
    w.db.Where("service_account_name = ? AND namespace = ?",
        sa.Name, sa.Namespace).Find(&pods)

    // 3. Create USES edges
    for _, pod := range pods {
        w.graphEngine.CreateEdge("ServiceAccount", sa.UID, "Pod", pod.UID, "USES", nil)
    }

    // 4. Find role bindings
    var bindings []models.RoleBinding
    w.db.Where("service_account_name = ? AND namespace = ?",
        sa.Name, sa.Namespace).Find(&bindings)

    // 5. Create HAS_ROLE edges
    for _, binding := range bindings {
        w.graphEngine.CreateEdge("ServiceAccount", sa.UID, "Role", binding.RoleUID, "HAS_ROLE", map[string]interface{}{
            "binding_name": binding.Name,
            "binding_type": binding.Type,
        })
    }

    return nil
}
```

---

## Performance

| Query Type | Avg Time | Notes |
|------------|----------|-------|
| Single-hop relationship | ~10ms | Direct edge traversal |
| 2-hop attack path | ~50ms | ServiceAccount → Pod → Secret |
| 3-hop attack path | ~200ms | Complex path finding |
| Shortest path | ~500ms | Full graph search |
| Blast radius (all resources) | ~300ms | Multiple traversals |

**Optimization**:
- Indexes on node properties (uid, name, namespace)
- Relationship indexes
- Query result caching
- Graph pruning (soft-deleted nodes excluded)

---

## Monitoring

### Prometheus Metrics

```
# Graph size
ksam_graph_vertices_total{type="ServiceAccount"}
ksam_graph_edges_total{type="USES"}

# Query performance
ksam_graph_query_duration_seconds{query_type="attack_path"}
ksam_graph_query_errors_total

# Correlator worker
ksam_correlator_processed_total
ksam_correlator_errors_total
```

### Health Checks

```bash
# Check graph size
SELECT * FROM cypher('ksam_security_graph', $$
  MATCH (n)
  RETURN labels(n) as type, count(n) as count
$$) as (type agtype, count agtype);

# Check edge count
SELECT * FROM cypher('ksam_security_graph', $$
  MATCH ()-[r]->()
  RETURN type(r) as edge_type, count(r) as count
$$) as (edge_type agtype, count agtype);
```

---

## Use Cases

### 1. Attack Path Analysis

**Question**: "How can an attacker access production secrets?"

```cypher
MATCH path = (sa:ServiceAccount {namespace: 'default'})
             -[*1..5]->
             (secret:Secret {namespace: 'production'})
RETURN path, length(path)
ORDER BY length(path)
LIMIT 10
```

**Insight**: Find shortest attack paths from low-privilege SAs to high-value secrets.

### 2. Lateral Movement Detection

**Question**: "What other pods can this compromised pod access?"

```cypher
MATCH (compromised:Pod {name: 'nginx-web'})
      -[:USES]-(sa:ServiceAccount)
      -[:USES]->(other_pod:Pod)
WHERE compromised <> other_pod
RETURN other_pod.name, other_pod.namespace
```

**Insight**: Understand blast radius if pod is compromised.

### 3. Privilege Escalation Chains

**Question**: "Which ServiceAccounts can escalate to cluster-admin?"

```cypher
MATCH path = (sa:ServiceAccount)
             -[:HAS_ROLE*1..3]->
             (role:Role {name: 'cluster-admin'})
RETURN sa.namespace, sa.name, length(path) as hops
ORDER BY hops
```

**Insight**: Detect multi-hop privilege escalation paths.

### 4. Namespace Boundary Violations

**Question**: "Can any ServiceAccount access resources outside its namespace?"

```cypher
MATCH (sa:ServiceAccount)-[:HAS_ROLE]->(role:Role)
      -[:CAN_ACCESS]->(resource:ResourceType)
WHERE role.type = 'ClusterRole'
RETURN sa.namespace, sa.name, role.name, resource.kind
```

**Insight**: Find cross-namespace access violations.

---

## Troubleshooting

### Graph Data Not Building

**Check Correlator Worker**:
```bash
kubectl logs -n ksam ksam-core-* | grep -i correlator
```

**Check Graph Exists**:
```sql
SELECT * FROM ag_graph WHERE name = 'ksam_security_graph';
```

**Rebuild Graph**:
```sql
-- Drop and recreate
SELECT drop_graph('ksam_security_graph', true);
SELECT create_graph('ksam_security_graph');

-- Trigger re-correlation
-- (Delete and re-sync resources from agent)
```

### Query Performance Slow

**Add Indexes**:
```sql
-- Index on vertex properties
CREATE INDEX ON ag_catalog.ksam_security_graph_vertex USING gin (properties);

-- Index on edge types
CREATE INDEX ON ag_catalog.ksam_security_graph_edge (label);
```

**Check Query Plan**:
```sql
EXPLAIN SELECT * FROM cypher('ksam_security_graph', $$
  MATCH (sa:ServiceAccount)-[:USES]->(pod:Pod)
  RETURN sa, pod
$$) as (sa agtype, pod agtype);
```

### Graph Data Inconsistent

**Verify Vertex Count**:
```sql
-- Check ServiceAccounts in PostgreSQL
SELECT COUNT(*) FROM serviceaccounts WHERE deleted_at IS NULL;

-- Check ServiceAccounts in graph
SELECT * FROM cypher('ksam_security_graph', $$
  MATCH (sa:ServiceAccount)
  RETURN count(sa)
$$) as (count agtype);
```

**Reconcile**:
```bash
# Trigger full re-sync
kubectl delete pods -n ksam -l app=ksam-core
```

---

## Development

### Local Testing

```bash
# Start PostgreSQL with AGE
docker run -d \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  apache/age:latest

# Connect and setup
psql -h localhost -U postgres -c "CREATE EXTENSION age;"
```

### Query Testing

```bash
# Test query via API
curl -X POST http://localhost:8080/api/v1/graph/query \
  -d '{
    "cypher": "MATCH (sa:ServiceAccount) RETURN sa LIMIT 5"
  }'
```

---

## Related Components

- [Correlator Worker](../core/#correlator-worker) - Builds graph data
- [Attack Path Analysis](../../07-features/attack-paths.md) - Uses graph queries
- [Core](../core/) - Provides graph API endpoints

---

## Future Enhancements

### Planned Features

1. **Advanced Queries**:
   - Community detection (find clusters of related resources)
   - Centrality analysis (find most connected nodes)
   - PageRank for risk scoring

2. **Temporal Graphs**:
   - Track relationship changes over time
   - Historical attack path analysis
   - Time-based queries

3. **Graph Visualization**:
   - D3.js interactive graphs
   - Zoom/pan/filter
   - Real-time updates

4. **Machine Learning**:
   - Anomaly detection in graph patterns
   - Predicted attack paths
   - Graph neural networks for risk scoring

---

**Last Updated**: December 16, 2025
**Status**: ✅ Production Ready
**Technology**: Apache AGE on PostgreSQL
