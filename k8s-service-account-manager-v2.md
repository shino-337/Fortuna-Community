# Kubernetes Service Account Manager (KSAM)

## 1. Project Overview

**Goal:**  
Develop a centralized tool to manage, audit, and visualize ServiceAccounts (SAs) across multiple Kubernetes clusters, ensuring visibility, compliance, and least-privilege principles.

**Problem Context:**  
- ServiceAccounts in Kubernetes are often unmanaged, reused, or overprivileged.  
- Current observability tools (Lens, K9s, Rancher) display SAs per-cluster but lack cross-cluster aggregation or risk assessment.  
- There is no unified view of token usage, bindings, and cluster roles associated with ServiceAccounts.

**Objective:**  
Build a **centralized management and observability system** that connects to multiple Kubernetes clusters, aggregates ServiceAccount data, and provides a visual graph to understand trust and privilege relationships.

---

## 2. System Requirements

### 2.1 Functional Requirements
1. **Discovery**
   - Enumerate all ServiceAccounts, RoleBindings, ClusterRoleBindings across connected clusters.
   - Detect relationships between SAs, namespaces, pods, and roles.
   - Monitor changes to SA definitions and bindings in real time.

2. **Centralized Management**
   - Allow admin to view, edit, delete, or disable ServiceAccounts centrally.
   - Support role revocation or token rotation from the dashboard.
   - Provide bulk operations (e.g., disable inactive SAs).

3. **Visualization**
   - Show relationships between ServiceAccounts, Roles, and Namespaces using a **graph view** (interactive, filterable).
   - Display privilege hierarchy and potential privilege escalations visually.

4. **Audit & Compliance**
   - Generate reports of SA permissions and usage.
   - Track token creation, usage patterns, and anomalies.
   - Integrate with external audit systems (Splunk, Elastic, Grafana Loki).

5. **Integration & Extensibility**
   - Connect to clusters using kubeconfig or via service mesh (API-based connectors).
   - Provide REST/gRPC API for external tools to query ServiceAccount data.
   - Support integration with OPA, Kyverno, or custom RBAC policy validators.

---

### 2.2 Non-Functional Requirements
- **Scalability:** Handle 100+ clusters efficiently.  
- **Security:** RBAC-controlled admin access; encrypt all communication (TLS).  
- **Performance:** Data sync under 10s latency for up to 10k ServiceAccounts.  
- **Reliability:** Auto-retry and backoff for API sync failures.  
- **Deployability:** Helm chart or Operator for quick installation.  
- **Observability:** Metrics via Prometheus, health endpoints, and audit logs.

---

## 3. Proposed Architecture

```
               ┌───────────────────────────┐
               │       KSAM Dashboard       │
               │(Web UI / Graph Visualization)│
               └────────────┬────────────────┘
                            │
                            ▼
                ┌────────────────────────┐
                │   KSAM Core Controller │
                │(API Server + Scheduler)│
                └────────────┬───────────┘
                             │
         ┌───────────────────┼───────────────────┐
         ▼                   ▼                   ▼
 ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
 │ Cluster A   │     │ Cluster B   │ ... │ Cluster N   │
 │ KSAM Agent  │     │ KSAM Agent  │     │ KSAM Agent  │
 └─────────────┘     └─────────────┘     └─────────────┘
                             │
                             ▼
                 ┌────────────────────────┐
                 │  Centralized Database   │
                 │ (PostgreSQL / Timescale)│
                 └────────────────────────┘
```

---

## 4. System Components

### 4.1 KSAM Agent (Cluster-Side)
- Lightweight DaemonSet deployed in each cluster.
- Periodically scans ServiceAccounts, RoleBindings, and Pod annotations.
- Sends updates to KSAM Core via gRPC or WebSocket.
- Can listen to Kubernetes watch API for real-time changes.
- Operates with least privilege — only requires read-only access to RBAC and ServiceAccount objects.

### 4.2 Data Flow (Detailed)
```
        ┌───────────────────────────┐
        │  Kubernetes Cluster (A)   │
        ├───────────────────────────┤
        │ SA, RoleBinding, CRB data │
        └───────────────┬───────────┘
                        │
                [1] KSAM Agent collects and watches SA/RBAC objects
                        │
                [2] Agent serializes data into gRPC or REST payloads
                        │
                [3] Secure channel (TLS) to KSAM Core Controller
                        │
                [4] KSAM Core normalizes and stores in DB
                        │
                [5] Dashboard queries KSAM Core (REST/WebSocket)
                        │
                [6] Visualization layer (Graph engine) renders data
```

**Data Storage Layers:**
- **PostgreSQL / TimescaleDB** for metadata and audit logs.
- **Redis** for caching active tokens and sync states.
- **Optional**: Neo4j / DGraph for advanced graph queries.

---

## 5. Dashboard & Visualization

### 5.1 Frontend Requirements
- Responsive dashboard using React + Tailwind + D3.js or Cytoscape.js.
- Display SA-RBAC relationships as **interactive graph**.
- Allow filtering by cluster, namespace, role, or privilege level.
- Show risk score (e.g., overly permissive roles, long-lived tokens).
- Provide time-based views of SA changes.

### 5.2 Backend APIs
- `/api/v1/clusters` – List clusters connected.  
- `/api/v1/serviceaccounts` – Query ServiceAccounts by filters.  
- `/api/v1/graph` – Return graph structure (nodes + edges).  
- `/api/v1/audit` – Return activity and change history.  

---

## 6. Integration with Kubernetes

### 6.1 Connection Methods
- Use **Kubeconfig aggregation** or **service-account impersonation** via API Server.
- Each agent authenticates with a token registered in the central KSAM Core.
- Communication uses **mTLS** (client certs) for identity verification.

### 6.2 Permissions
Agent requires minimal RBAC rights:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ksam-agent-reader
rules:
  - apiGroups: [""]
    resources: ["serviceaccounts", "pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["roles", "rolebindings", "clusterroles", "clusterrolebindings"]
    verbs: ["get", "list", "watch"]
```

---

## 7. Suggested Technology Stack

| Layer | Technology | Description |
|-------|-------------|-------------|
| Agent | Go (client-go, gRPC) | Efficient API watcher |
| Core Controller | Go / Python (FastAPI) | Central management + scheduler |
| Database | PostgreSQL, Redis, Neo4j | Data + graph relationships |
| Dashboard | React + Tailwind + Cytoscape.js | Graph visualization UI |
| Auth | OIDC / Keycloak / Dex | Central authentication |
| CI/CD | Helm + ArgoCD | Deployment and updates |

---

## 8. Deployment & Resource Optimization

- **Helm chart** with configurable namespaces, limits, and securityContext.  
- KSAM Agent: 50–100MB memory footprint, runs as DaemonSet.  
- KSAM Core: Horizontally scalable API pods (autoscaled).  
- Use **async queue (Kafka/NATS)** for scalable sync if clusters >50.  
- Avoid API rate-limit conflicts by using **shared informer cache**.  
- Agents use exponential backoff to reduce network load.

---

## 9. Security Considerations

- All communications encrypted with TLS (mTLS between agent ↔ core).  
- API Gateway enforces role-based access control (RBAC).  
- Tokens and credentials stored in HashiCorp Vault or sealed-secrets.  
- Signed container images (cosign, sigstore).  
- Audit trails for all user and API actions.  

---

## 10. Future Enhancements

- AI-based anomaly detection for abnormal ServiceAccount activity.  
- Integration with CSPM tools (Prisma, Wiz, Orca).  
- Risk scoring based on privilege levels and usage frequency.  
- Drift detection: auto-remediation when an SA deviates from baseline.  
- CLI tool for automation and offline audit.  

---

## 11. Deliverables

- KSAM Agent (Go)  
- KSAM Core Controller (API + Scheduler)  
- KSAM Dashboard (Web UI)  
- Helm Chart for deployment  
- Integration with Prometheus, Grafana, Loki  
- Documentation and security hardening guide  
