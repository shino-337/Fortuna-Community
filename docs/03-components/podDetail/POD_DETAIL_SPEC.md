# Pod Detail Specification

## 1. Purpose

Define the functional, data, and UI requirements for the Pod Detail view in the Resource module.

This view must support:
- Security investigation
- Runtime troubleshooting
- Supply chain verification
- Compliance validation
- Incident root cause analysis

The Pod Detail page is NOT a simple viewer. It must function as a security-grade inspection console.

---

# 2. High-Level Layout

## Page Structure

Pod Detail Page
│
├── Header (Identity & Quick Risk)
├── Tabs:
│   ├── Overview
│   ├── Security
│   ├── Supply Chain
│   ├── Network
│   ├── Resources
│   ├── Config
│   ├── Events
│   └── Compliance (optional)
│
└── Action Bar (top-right)

---

# 3. Header Section (Sticky)

## Required Fields

| Field | Description |
|-------|------------|
| Pod Name | Full name |
| Namespace | Namespace |
| Cluster | Cluster name |
| Status | Running / Pending / Failed / CrashLoopBackOff |
| Node | Node name |
| Pod IP | Pod IP address |
| Start Time | Pod start time |
| Uptime | Calculated runtime |
| Restart Count | Total restarts |
| Risk Score | Aggregated risk score |
| Severity Badge | Critical / High / Medium / Low |

## Action Buttons

- View YAML
- Exec (if permitted)
- Delete Pod
- Restart (if controller-managed)
- Export JSON
- Trigger Re-scan

---

# 4. Overview Tab

## 4.1 Identity & Ownership

| Field | Description |
|-------|------------|
| Owner Type | Deployment / StatefulSet / Job / CronJob |
| Owner Name | Controller name |
| ReplicaSet | If applicable |
| Revision | Deployment revision |
| QoS Class | Guaranteed / Burstable / BestEffort |

## 4.2 Containers Summary

For each container:

- Container Name
- Image (tag)
- Image Digest (sha256)
- State (Running / Waiting / Terminated)
- Restart Count
- Ports
- Ready (true/false)

Clickable → Opens container side panel.

---

# 5. Security Tab

## 5.1 Pod-Level Security Context

| Field |
|-------|
| runAsUser |
| runAsGroup |
| runAsNonRoot |
| fsGroup |
| seccompProfile |
| SELinux context |
| AppArmor profile |
| hostNetwork |
| hostPID |
| hostIPC |

## 5.2 Container-Level Security

For each container:

- privileged
- allowPrivilegeEscalation
- readOnlyRootFilesystem
- capabilities (added / dropped)
- securityContext overrides

Flag visually if:
- privileged = true
- capabilities include SYS_ADMIN
- allowPrivilegeEscalation = true

## 5.3 Service Account

- ServiceAccount name
- Token automount (true/false)
- Linked Roles (if resolvable)
- RBAC risk indicator

---

# 6. Supply Chain Tab

## 6.1 Image Metadata

| Field |
|-------|
| Registry |
| Repository |
| Tag |
| Digest |
| Pull Policy |
| Last Pull Time |

## 6.2 Image Integrity

- Signature status (Verified / Not Verified)
- SBOM available (yes/no)
- Vulnerability summary:
  - Critical
  - High
  - Medium
  - Low
- Fix version available (yes/no)

## 6.3 Provenance (if available)

- Build system
- Build timestamp
- Commit SHA
- Builder identity

---

# 7. Network Tab

## 7.1 Ports

For each container:
- Container Port
- Protocol
- Host Port (if mapped)

## 7.2 Service Exposure

- Linked Services
- Service Type (ClusterIP / NodePort / LoadBalancer)
- External IP (if any)
- Public exposure indicator

## 7.3 Ingress

- Ingress Name
- Host
- Path
- TLS enabled (yes/no)

## 7.4 Network Policy

- Applied NetworkPolicies
- Isolated (true/false)
- Egress allowed?
- Ingress allowed?

Highlight:
- No NetworkPolicy
- Public exposure + High severity vulnerability

---

# 8. Resources Tab

## 8.1 Requested vs Limits

For each container:

- CPU Request
- CPU Limit
- Memory Request
- Memory Limit
- Ephemeral Storage

## 8.2 Runtime Metrics

- Current CPU usage
- Current Memory usage
- CPU throttling events
- OOMKilled count

Display mini time-series graph (optional).

---

# 9. Config Tab

## 9.1 Volumes

For each volume:
- Type (ConfigMap / Secret / PVC / hostPath / EmptyDir)
- Name
- Mount Path

Flag:
- hostPath usage
- Secret mounted as file

## 9.2 Environment Variables

- Key
- Source (literal / ConfigMap / Secret / Downward API)
- Mask sensitive values

## 9.3 Secret Exposure Analysis

- Secret name
- Keys used
- Secret risk level (if scanned)

---

# 10. Events Tab

## 10.1 Kubernetes Events

Chronological table:

- Timestamp
- Type (Normal / Warning)
- Reason
- Message
- Source

## 10.2 Security Events (if runtime agent exists)

- Suspicious process execution
- Exec into pod event
- File modification anomaly
- Reverse shell detection
- Crypto miner pattern

---

# 11. Compliance Tab (Optional)

## 11.1 Policy Violations

- Policy name
- Severity
- Category
- Status (Open / Resolved)

## 11.2 Benchmark Mapping

- CIS control reference
- NIST control reference
- Internal policy mapping

---

# 12. Risk Aggregation Logic

Risk Score must be derived from:

- CVE severity
- Privileged mode
- Public exposure
- Secret misuse
- Compliance violation
- Drift from Git baseline

Risk formula must be documented separately.

---

# 13. Drift Detection (If GitOps integrated)

Display:

- Drift status (In Sync / Drifted)
- Changed fields
- Last Git commit
- Last apply timestamp

---

# 14. Filtering & Search Requirements

User must be able to:

- Filter containers
- Filter vulnerabilities by severity
- Filter events by type
- Expand/collapse sections

---

# 15. Performance Requirements

- Load time < 2 seconds for 95th percentile
- Data fetched in parallel
- Lazy load heavy sections (Events, SBOM)

---

# 16. Security Requirements

- RBAC enforced per cluster
- Mask sensitive fields
- Audit log on:
  - Exec
  - Delete
  - Export
  - View secret

---

# 17. Non-Goals

This page must NOT:

- Allow editing raw YAML directly (view-only)
- Overload with full SBOM tree (link to SBOM page)
- Display redundant cluster-level metadata

---

# 18. MVP Mandatory Fields

If implementing minimal version, must include:

- Owner reference
- Image digest
- ServiceAccount
- Privileged flag
- Public exposure
- Secret usage
- Vulnerability summary

Without these, this is not considered a security-ready Pod Detail view.

---

# 19. Implementation Notes (Backend)

- **Database:** Migration 068 adds to `pods` table: `pod_ip`, `start_time`, `restart_count`, `owner_kind`, `owner_name`, `replica_set_name`, `qos_class`. Core model `Pod` and type `NullTime` (for `start_time`) in `core/pkg/models`.
- **Agent:** Syncer sends Pod Detail fields in sync payload: `podIP`, `startTime` (RFC3339), `restartCount`, `ownerKind`, `ownerName`, `replicaSetName`, `qosClass`. QoS and owner refs derived from `corev1.Pod` in `agent/internal/syncer`.
- **API:** `GET /api/v1/pods/:id` and `GET /api/v1/pods/:podUid` return these fields; dashboard type `PodWithRisk` includes them.

---

# 20. Pod Data Flow (Agent → Core → Database → Dashboard)

End-to-end flow of pod information from Kubernetes to the Dashboard.

## 20.1 Overview Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│  Kubernetes API (cluster)                                                                 │
│  • Pods, ServiceAccounts, Roles, RoleBindings, ClusterRoles, ClusterRoleBindings,        │
│    Deployments, ReplicaSets                                                               │
└───────────────────────────────────┬─────────────────────────────────────────────────────┘
                                    │ List (watch not used for sync)
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│  AGENT (per node / per cluster)                                                           │
│  • Config: CORE_HTTP_ENDPOINT, SYNC_INTERVAL (default 30s)                                │
│  • Syncer: buildPayload() → list Pods (and RBAC/SA/Deploy/RS) from K8s API                │
│  • PodPayload: name, namespace, uid, phase, serviceAccountName, nodeName, hostNetwork,    │
│    hostPID, hostIPC, automountServiceAccountToken, podSecurityContext, containers,       │
│    volumes, tolerations, affinity, podIP, startTime, restartCount, ownerKind, ownerName,   │
│    replicaSetName, qosClass                                                               │
│  • SyncOnce() → POST { clusterId, cluster?, agent?, data: { isFullSync, pods, ... } }      │
└───────────────────────────────────┬─────────────────────────────────────────────────────┘
                                    │ HTTP POST /api/v1/agent/sync
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│  CORE (API)                                                                               │
│  • SyncDataFromAgent(agent_handlers.go): bind JSON → normalize clusterId →                │
│    AgentService.SyncData(clusterID, clusterName, source, k8sVersion, distribution, data)   │
│  • If isFullSync: process ServiceAccounts, Roles, RoleBindings, ClusterRoles,            │
│    ClusterRoleBindings, Pods, Deployments, ReplicaSets; cleanupStalePods; trigger         │
│    HistoricalRiskEvaluator + PodCapabilityEngine (async)                                 │
│  • If delta: process pods only when data.pods present                                     │
└───────────────────────────────────┬─────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│  CORE (AgentService)                                                                      │
│  • processSyncedPods(clusterID, data, isFullSync):                                        │
│    - For each data.pods[]: parse name, namespace, uid, phase, serviceAccountName,         │
│      nodeName, hostNetwork/hostPID/hostIPC, automount, containers (JSON), volumes,      │
│      tolerations, affinity, podSecurityContext, containerSecurityContexts,                │
│      podIP, startTime (RFC3339→NullTime), restartCount, ownerKind, ownerName,            │
│      replicaSetName, qosClass                                                             │
│    - Upsert by (cluster_id, uid): Create new | Update existing | Restore soft-deleted    │
│    - On update: compare all fields (including pod detail) to set "changed"                │
│    - After full sync: soft-delete pods not in payload; dedupe by UID                      │
│    - Per pod: EnsureActiveInstance (pod_instances), evaluatePodCapabilities (PCE)          │
└───────────────────────────────────┬─────────────────────────────────────────────────────┘
                                    │ GORM Create/Updates/Save
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│  DATABASE (PostgreSQL)                                                                    │
│  • Table: pods (id, cluster_id, uid, name, namespace, service_account, containers,       │
│    image_digests, pod_security_context, container_security_contexts, volume_mounts,     │
│    volumes, tolerations, affinity, host_network, host_pid, host_ipc,                       │
│    automount_service_account_token, node_name, phase, pod_ip, start_time, restart_count, │
│    owner_kind, owner_name, replica_set_name, qos_class, created_at, updated_at, deleted_at) │
│  • Other: clusters, insights (riskCount by resource_uid), pod_instances, pod_capabilities │
└───────────────────────────────────┬─────────────────────────────────────────────────────┘
                                    │
                                    │  Dashboard reads via Core HTTP API
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│  CORE (Read APIs)                                                                         │
│  • GET /api/v1/pods?cluster=&namespace=&node=&page=&pageSize=  → GetPods: list Pod +    │
│    riskCount per UID (from insights), paginated                                           │
│  • GET /api/v1/pods/:id         → GetPod: one Pod by primary key, Preload(Cluster),      │
│    riskCount                                                                              │
│  • GET /api/v1/pods/:podUid → GetPodByUID: one Pod by uid, same shape                 │
│  • GET /api/v1/resources?kind=&namespace=&cluster= → GetResources: normalized list       │
│    (kind, name, namespace, uid, clusterId); for Pod reads from pods table                 │
└───────────────────────────────────┬─────────────────────────────────────────────────────┘
                                    │ JSON response (full Pod model + riskCount)
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│  DASHBOARD (Frontend)                                                                     │
│  • API base: VITE_CORE_API_URL + /api/v1 or /api/v1 (same origin)                         │
│  • Resources (Pod tab): api.getPods({ cluster, namespace, page, pageSize }) → table       │
│    (name, namespace, node, status←phase, risk, View→/resources/pods/:id or /uid/:uid)      │
│  • PodDetail: api.getPod(id) or api.getPodByUid(uid) → header (name, risk, namespace,     │
│    nodeName), cards (status←phase, riskCount, serviceAccount, createdAt), Overview       │
│    (namespace, node, SA, UID), SBOM tab (api.getPodSbom), Risks tab (getPodRiskReport)   │
│  • Type: PodWithRisk (id, clusterId, name, namespace, uid, nodeName, serviceAccount,       │
│    status/phase, riskCount, createdAt, podIP, startTime, restartCount, ownerKind,         │
│    ownerName, replicaSetName, qosClass)                                                   │
│  • getPods() maps API phase → status for list; single-pod responses pass full object      │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

## 20.2 Step-by-Step Summary

| Step | Component | Action |
|------|-----------|--------|
| 1 | Agent | Periodically (SYNC_INTERVAL) calls `buildPayload()`: lists Pods (and RBAC/SA/Deploy/RS) from K8s API, builds `SyncPayload` with `data.pods[]` (PodPayload with all fields including POD_DETAIL_SPEC). |
| 2 | Agent | `SyncOnce()` POSTs JSON to Core `POST /api/v1/agent/sync`. |
| 3 | Core | `SyncDataFromAgent` receives body, normalizes clusterId, calls `AgentService.SyncData(..., req.Data)`. |
| 4 | Core | `SyncData` determines isFullSync/isDeltaSync; processes SAs; if full sync processes RBAC then `processSyncedPods(clusterID, data, true)`, then Deployments/ReplicaSets, cleanupStalePods, triggers risk/PCE. On delta, only processes pods if `data.pods` present. |
| 5 | Core | `processSyncedPods` iterates `data["pods"]`, parses each into `models.Pod` (including podIP, startTime, restartCount, owner*, qosClass), upserts by (cluster_id, uid); full sync then soft-deletes pods not in payload. |
| 6 | DB | Rows in `pods` (and related tables) updated/created. |
| 7 | Dashboard | User opens Resources → Pod tab: `getPods()` → `GET /api/v1/pods` → Core `GetPods` → query pods + riskCount → JSON. |
| 8 | Dashboard | User clicks pod: navigate to `/resources/pods/:id` or `/resources/pods/uid/:uid` → PodDetail loads `getPod(id)` or `getPodByUid(uid)` → `GET /api/v1/pods/:id` or `GET /api/v1/pods/:podUid` → Core returns full Pod (with Cluster preload) + riskCount. |
| 9 | Dashboard | PodDetail renders header, status (phase), riskCount, serviceAccount, nodeName, Overview; SBOM and Risks tabs call getPodSbom / getPodRiskReport. |

## 20.3 Data Sources for Pod Detail View

| Field shown | Source |
|-------------|--------|
| Name, Namespace, UID, Node, Status (phase), Service Account, Created | `pods` table (from agent sync). |
| Risk count / badge | `insights` table (active count by resource_uid). |
| POD_DETAIL_SPEC (podIP, startTime, restartCount, ownerKind, ownerName, replicaSetName, qosClass) | `pods` table (agent sends, core stores in processSyncedPods). |
| SBOM / vulnerability summary | SBOM and CVE pipeline (agent sends SBOM via gRPC; core stores, aggregates). |
| Related risks | `insights` + risk report API. |
| Cluster name | `Cluster` preloaded on Pod (GET /pods/:id or by-uid). |