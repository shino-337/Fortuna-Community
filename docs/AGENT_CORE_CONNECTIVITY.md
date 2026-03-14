# Agent – Core connectivity

## Why Agent cannot connect after rollout

Agent connects to Core over **gRPC (port 9090)** with **mTLS**. After `rollout restart` of Core and/or Agent, connection can fail until the following are true.

### 1. Core pod is Running and Ready

- Core runs only on the control-plane node.
- Core becomes **Ready** when HTTP (8080) and gRPC (9090) are listening. Readiness checks `127.0.0.1:9090`.
- Core startup blocks on: DB connect, migrations, then gRPC/HTTP. So Core can take **1–2 minutes** to become Ready after deploy/restart.

### 2. Service has endpoints

- Only **Ready** Core pods are in the `fortuna-core` Service endpoints.
- If Core is not Ready, endpoints are empty and Agent gets **connection refused**.

### 3. DNS

- Agent uses `fortuna-core.fortuna.svc.cluster.local:9090`. This must resolve from pods in `fortuna` namespace.

### 4. mTLS

- Core server cert: `fortuna-core-tls`. Agent client cert: `fortuna-agent-tls`. Both must be signed by **fortuna-ca-cert**. Otherwise TLS handshake fails.

### 5. Rollout order

- After Core restart, Core is Not Ready for 1–2 min. Agent **retries every 15s** and does not exit; once Core is Ready, Agent connects.

## SBOM payload (Agent → Core)

Agent sends SBOM findings via gRPC (`SendSBOMFinding`). The payload includes:

- **Pod/image identity:** pod_uid, namespace, container name, image name/digest/tag.
- **Packages:** each with `name`, `version`, `type` (e.g. deb, npm, generic), and **PURL** (e.g. `pkg:generic/coredns@1.11.0` for distroless images).
- **SBOM-level metadata (Finding #8.4):**
  - **sbom_source:** `parsers` | `distroless-heuristic` | `label-metadata` — how the SBOM was produced.
  - **confidence:** `low` | `medium` | `high` — inference reliability (e.g. medium when version from OCI labels).

Core persists these to `sboms` and `sbom_components` (including `sbom_source`, `confidence`, `purl`). The CVE matcher uses **sbom_source** and **confidence**: for `distroless-heuristic` or `label-metadata` with confidence &lt; high, when the OSV/Postgres dataset has no match, Core tries the **NVD API** fallback so heuristic SBOMs still get CVE coverage. See `docs/03-components/sbom/DISTROLESS_SBOM_SPEC.md` and Remediation Plan Finding #8.

## Trace from Agent to insight (Finding #5.2)

Agent can send **X-Correlation-ID** (HTTP sync) or **x-correlation-id** (gRPC metadata). Core stores it in **audit_logs.trace_id** for every audit entry created during that sync (create/update/delete of SAs, Roles, Pods, etc.). This links a single Agent request to all audit logs produced by that sync, so you can trace from Agent → Core sync → audit log (and later to insight/CVE worker when they log the same ID). API: `GET /api/v1/audit/logs` returns `traceId` in each log entry when present.

## Commands

```bash
kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o wide
kubectl get endpoints fortuna-core -n fortuna
kubectl logs -n fortuna -l app.kubernetes.io/component=core --tail=100
./scripts/verify/verify-agent-core-connectivity.sh
```

## Agent log Diagnostic (after code change)

On connect failure, Agent logs a **Diagnostic** line:

- **connection refused** – Core not Ready or gRPC not listening; check Core pod and endpoints.
- **i/o timeout** – Network/DNS; try nslookup from agent pod.
- **no such host** – DNS; ensure Service exists in `fortuna`.
- **tls: / handshake / certificate** – mTLS; ensure agent and Core certs from same CA.
