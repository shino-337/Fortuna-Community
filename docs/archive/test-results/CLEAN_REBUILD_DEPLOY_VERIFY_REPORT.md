# Clean / Rebuild / Deploy – Verification Report

**Date:** 2026-02-22  
**Script:** `scripts/pipeline/full-clean-database-rebuild-deploy.sh` (with `--db-reset`)

---

## 1. Script execution (observed)

### Run 1: Full clean + rebuild + deploy (timed out after ~10 min)

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 1 Clean | OK | Port-forwards killed, fortuna images removed from containerd, prune |
| Phase 1b DB | OK | `reset_database_full.sql` applied – all tables dropped |
| Phase 2 Rebuild | OK | core, agent, dashboard built (nerdctl → containerd) |
| Phase 2b Push | OK | Images pushed to 192.168.56.100, 192.168.56.101 |
| Phase 2a Addons | OK | kube-proxy and CoreDNS already running; sleep 15s |
| Phase 2c StorageClass | (not reached before timeout) | — |
| Phase 3a CNI | (not reached) | — |
| Phase 3 Deploy | (not reached) | — |

### Run 2: Deploy only (`--skip-clean --skip-rebuild`)

| Phase | Status | Notes |
|-------|--------|-------|
| Phase 2a Addons | OK | kube-proxy, CoreDNS running |
| Phase 2c StorageClass | OK | local-path available; sleep 10s |
| Phase 3a CNI | OK | Flannel VXLAN verified; restart + 15s wait; sleep 10s |
| Phase 3 Deploy | OK | deploy-fortuna-robust.sh ran |
| Pre-deployment checks | OK | kubectl, nerdctl, cluster, CoreDNS, kube-proxy, StorageClass |
| Step 3 Cleanup | OK | Old Core deployment/service/pods deleted; sleep 8s |
| Step 4 Infra | OK | Postgres/NATS already exist |
| Step 4b CNI | OK | Flannel fix run again (DNS reachable from Flannel pod) |
| Step 7 RBAC | OK | fortuna-rbac applied; sleep 5s |
| Step 8 Core | OK | Deployment + service created; DATABASE_URL IP fallback; Core ready |
| Step 8a/8b | OK | Core has endpoints; DNS resolves fortuna-core |
| Step 9 Agent | OK | DaemonSet rollout success; sleep 10s |
| Step 9b Dashboard | (likely ran after timeout) | — |
| Step 10 Rollout restart | (likely ran after timeout) | — |

---

## 2. K8s state (after deploy)

- **Nodes:** k8s-master (control-plane), k8s-worker01 – Ready  
- **kube-system:** kube-proxy (2), CoreDNS (2) – Running  
- **StorageClass:** local-path – present  
- **fortuna namespace:**
  - fortuna-core: 1/1 Running (10.244.0.225)
  - fortuna-agent: 2/2 (master + worker01) Running
  - fortuna-dashboard: 1/1 Running
  - postgres: 1/1 Running (postgres-5484f7745d-gsr4f)
  - nats: 3/3 Running  
- **PVCs:** postgres-pvc, data-nats-* – Bound (local-path)

---

## 3. Core logs

- Migrations ran on startup (DB was empty after `--db-reset`).
- Core is writing to DB: `pod_capabilities`, `role_bindings`, `cluster_roles` (cluster_id `sha256-c93f5c6cb0e57f8f`).
- Health: `/ready`, `/healthz` return 200.

---

## 4. Agent logs

- **Sync:** `[Syncer] ✅ Full sync completed` on both agent pods (master and worker01).
- **SBOM:** Some SendSBOMFinding RPC failures (`error reading server preface: EOF`) – expected when using IP fallback with TLS disabled (gRPC TLS mismatch).

---

## 5. Database (Postgres)

- **clusters:** 1 row – `id=sha256-c93f5c6cb0e57f8f`, `name=cluster-c93f5c6c`, `status=active`, `last_sync` set.
- **agents:** 0 rows – Agent registration is done via gRPC (RegisterAgent). With TLS disabled and IP fallback, gRPC can fail while HTTP sync still succeeds, so cluster is created by sync but agents table is not populated.

---

## 6. Conclusions

1. **Pipeline script:** Phase 2a (addons), 2c (StorageClass), 3a (CNI), and deploy (infra, RBAC, Core, Agent) ran as designed. Pre-checks (kube-proxy, StorageClass) passed. Sleeps and waits are in place.
2. **Clean + rebuild:** Image clean, DB reset, rebuild, and push to nodes completed successfully in the first run.
3. **Deploy:** Core and Agent are running; cluster record is created via HTTP sync; Core runs migrations and serves traffic.
4. **Agents table empty:** With DNS fallback (IP + TLS disabled), gRPC RegisterAgent can fail; only HTTP sync runs. To get agents in DB and in dashboard, use DNS + mTLS (fix pod network/DNS and re-enable TLS) or accept IP fallback with cluster data only.

---

## 7. Recommendations

- For long full runs, use background mode to avoid timeout:  
  `RUN_ASYNC=1 ./scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset`  
  then `tail -f /tmp/clean-rebuild-deploy.log`.
- Restore DNS + mTLS for Agent so RegisterAgent succeeds and dashboard shows agents:  
  `kubectl set env daemonset/fortuna-agent -n fortuna CORE_GRPC_ENDPOINT="fortuna-core.fortuna.svc.cluster.local:9090" TLS_ENABLED="true"`  
  then `kubectl rollout restart daemonset/fortuna-agent -n fortuna`.
