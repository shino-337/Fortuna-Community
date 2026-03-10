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
