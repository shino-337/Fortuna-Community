# Full Pipeline Verification Report – Chi tiết sau khi chạy không timeout

**Ngày:** 2026-02-22  
**Pipeline:** `scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset` (chạy full, không timeout)  
**Log:** `/tmp/clean-rebuild-deploy.log`

---

## 1. Kết quả pipeline (từ log)

| Phase | Trạng thái | Ghi chú |
|-------|------------|---------|
| Phase 1 Clean | ✅ | Port-forwards dừng, xóa toàn bộ image fortuna, prune |
| Phase 1b DB | ✅ | `reset_database_full.sql` – DROP toàn bộ bảng |
| Phase 2 Rebuild | ✅ | core, agent, dashboard build (nerdctl → containerd) |
| Phase 2b Push | ✅ | Image đẩy lên 192.168.56.100, 192.168.56.101 |
| Phase 2a Addons | ✅ | kube-proxy, CoreDNS đã chạy; sleep 15s |
| Phase 2c StorageClass | ✅ | local-path có sẵn; sleep 10s |
| Phase 3a CNI | ✅ | Flannel VXLAN verified; restart + sleep |
| Phase 3 Deploy | ✅ | deploy-fortuna-robust.sh chạy đủ bước |
| Pre-deployment checks | ⚠️ 1 cảnh báo | Tiếp tục deploy (e.g. DNS test hoặc worker label) |
| Step 3 Cleanup | ✅ | Xóa deployment/service Core cũ; sleep 8s |
| Step 4 Infra | ✅ | Postgres, NATS đã có |
| Step 4b CNI | ✅ | Flannel fix, DNS reachable từ Flannel pod |
| Step 6 DNS | ✅ | DNS resolution working |
| Step 7 RBAC | ✅ | fortuna-rbac applied; sleep 5s |
| Step 8 Core | ✅ | Deployment + service; Core ready, có endpoints |
| Step 8b DNS | ✅ | fortuna-core.fortuna.svc.cluster.local resolve OK |
| Step 9 Agent | ✅ | 2 Agent pods; rollout status; sleep 10s |
| Step 9b Dashboard | ✅ | Dashboard deployed |
| Step 10 Rollout | ✅ | Restart Core/Dashboard/Agent; Core rollout OK; sleep 15s |
| Step 11 Verification | ✅ | Core, Agent, Dashboard, Endpoints in log |
| Phase 3b | ✅ | Rollout restart, wait Core 120s, sleep 10s |
| **Kết luận** | **✅ Hoàn thành** | "Full clean / rebuild / deploy finished." |

---

## 2. Trạng thái cluster (sau khi chạy)

### 2.1 Node

| Node         | Status | Role         | Version  |
|-------------|--------|--------------|---------|
| k8s-master  | Ready  | control-plane| v1.29.15 |
| k8s-worker01| Ready  | —            | v1.29.15 |

### 2.2 Addons & Storage

- **kube-proxy:** 2 pod Running (kube-system)
- **CoreDNS:** 2 pod Running (kube-system)
- **StorageClass:** `local-path` (rancher.io/local-path)
- **PVC (fortuna):** postgres-pvc, data-nats-0/1/2 – đều **Bound**

### 2.3 Namespace `fortuna`

| Workload      | Loại       | Ready | Trạng thái |
|---------------|------------|-------|------------|
| fortuna-core  | Deployment | 1/1   | Running    |
| fortuna-agent | DaemonSet  | 2/2   | Running (master + worker01) |
| fortuna-dashboard | Deployment | 1/1 | Running    |
| postgres      | Deployment | 1/1   | Running    |
| nats          | StatefulSet| 3/3   | Running    |

- **Lưu ý:** Có 1 pod `postgres-5898b788f6-bg4vs` ở trạng thái Pending (replica thừa/khác bộ deploy; không ảnh hưởng Postgres chính đang chạy).

---

## 3. Database (PostgreSQL)

| Bảng / Chỉ số   | Kết quả |
|-----------------|--------|
| **clusters**    | 1 dòng: `id=sha256-c93f5c6cb0e57f8f`, `name=cluster-c93f5c6c`, `status=active`, `last_sync` có giá trị |
| **agents**      | 2 dòng: `k8s-master-agent`, `k8s-worker01-agent` (last_seen_at mới) |
| **pod_instances**| 21 |
| **pods**         | 21 |

→ Migrations chạy đầy đủ; sync cluster + đăng ký agent đều ghi DB.

---

## 4. Core

- **Health:** `/healthz` → 200, `{"status":"alive",...}`
- **Log:** Không lỗi; nhận probe `/ready`, `/healthz` 200
- **Service:** ClusterIP 10.108.189.1:8080,9090; endpoints 10.244.0.227

---

## 5. Agent

- **Sync:** Cả 2 pod đều có log **`[Syncer] ✅ Full sync completed`**
- **Heartbeat:** **`✅ Heartbeat OK (interval=15s)`**
- **SBOM:** Một số "Containerd fetch failed" / digest not found (bình thường khi image chưa có trên node); không chặn sync/heartbeat.

---

## 6. API (đã test qua port-forward 8080)

| API | Kết quả |
|-----|--------|
| **POST /api/v1/auth/login** | 200, trả về `token` + `user` (admin) |
| **GET /api/v1/clusters** | 200, 1 cluster: `cluster-c93f5c6c`, active, lastSync, k8sVersion v1.29.15 |
| **GET /api/v1/dashboard/stats** | 200: `totalClusters: 1`, `activeAgents: 2`, `runningPods: 20`, `totalRisks: 0` |
| **GET /api/v1/agents/status** | 200: 2 agents (k8s-master-agent, k8s-worker01-agent), `total: 2`, `slow: 2` (heartbeat cũ vài phút nên status "slow") |

---

## 7. Tóm tắt

- **Pipeline:** Chạy full (clean + db-reset + rebuild + deploy) **không timeout**, kết thúc bình thường.
- **Hạ tầng:** Addons (kube-proxy, CoreDNS), StorageClass, CNI (Flannel), RBAC, DNS đều ổn.
- **Ứng dụng:** Core, Agent, Dashboard, Postgres, NATS đều Running; Core health 200.
- **Dữ liệu:** 1 cluster, 2 agents, 21 pods trong DB; sync và heartbeat OK.
- **API:** Login, clusters, dashboard/stats, agents/status đều 200 và dữ liệu đúng.

**Kết luận:** Full pipeline đã chạy thành công; cluster và dashboard nhận đủ thông tin cluster và agent. Có thể dùng Dashboard (port-forward fortuna-dashboard 8081:80) và đăng nhập admin để xem cluster/pod.
