# Clean, Rebuild, Redeploy và Verify

Quy trình: clean → rebuild images → deploy (kèm DB reset nếu cần) → rollout → **monitor** và **verify** (migrations, seed khi FORTUNA_ENABLE_SEED_DATA=true, APIs, E2E, dữ liệu thực tế).

---

## 1. Chạy pipeline (clean + rebuild + deploy)

Pipeline mất khoảng **15–30 phút** (build core/agent/dashboard, push images, deploy). Có hai cách:

### Cách A: Chạy nền (khuyến nghị – tránh timeout)

```bash
cd /path/to/KSAM
export NAMESPACE=fortuna

# Chạy nền; log ghi vào /tmp/clean-rebuild-deploy.log
RUN_ASYNC=1 bash scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset
```

Theo dõi tiến độ:

```bash
tail -f /tmp/clean-rebuild-deploy.log
```

- **Phase 1:** Clean images, E2E ns, prune.  
- **Phase 1b:** DB reset (DROP tables) nếu dùng `--db-reset`.  
- **Phase 2:** Rebuild core, agent, dashboard (nerdctl → containerd).  
- **Phase 2b:** Push images sang các node.  
- **Phase 2a/2c:** Cluster addons, StorageClass.  
- **Phase 3:** Deploy (infra, RBAC, core, agent, dashboard).  
- **Phase 3b:** Rollout restart Core, Dashboard, Agent.

Khi thấy dòng `Full clean / rebuild / deploy finished.` thì pipeline xong.

### Cách B: Chạy trực tiếp (có thể bị timeout với IDE/script)

```bash
NAMESPACE=fortuna bash scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset
```

---

## 2. Kiểm tra sau khi deploy (monitor + dữ liệu thực tế)

Sau khi pipeline báo xong, chạy script verify để:

- Kiểm tra pods (Core, Agent, Dashboard, Postgres, NATS).  
- Xem Core logs: migrations 050/051/061 (built-in seed).  
- Gọi API: **capability-metadata** (số bản ghi Capability Catalog), **health/dashboard-data-integrity** (crossChecks).  
- Chạy E2E **pod-delete cleanup** (APIs PCE/risk, tạo pod → xóa → kiểm tra cleanup).  
- Ghi báo cáo vào `docs/test-results/verify-after-deploy-<timestamp>.md`.

```bash
cd /path/to/KSAM
export NAMESPACE=fortuna

# Có thể chỉ định thư mục báo cáo
export REPORT_DIR=docs/test-results

bash scripts/verify/verify-after-deploy-and-e2e.sh
```

Kết quả in ra màn hình và ghi vào file trong `REPORT_DIR` với nội dung:

- Trạng thái pods.  
- Số bản ghi **capability_metadata** (Capability Catalog).  
- CrossChecks từ **dashboard-data-integrity** (podsCount, insightsCount, activeAgentsCount, cvesCount).  
- Kết quả E2E pod-delete cleanup.

---

## 3. Monitor thủ công (tùy chọn)

- **Pods:**  
  `kubectl get pods -n fortuna -o wide -w`

- **Core logs (migrations / seed):**  
  `kubectl logs -n fortuna -l app=fortuna-core -f --tail=200`

- **Agent sync:**  
  `kubectl logs -n fortuna -l app=fortuna-agent -f --tail=100`

- **API nhanh (trong cluster):**  
  `kubectl exec -n fortuna deploy/fortuna-core -- curl -s http://localhost:8080/api/v1/health/dashboard-data-integrity`  
  `kubectl exec -n fortuna deploy/fortuna-core -- curl -s http://localhost:8080/api/v1/capability-metadata | python3 -c "import sys,json; d=json.load(sys.stdin); print('capability_metadata count:', len(d.get('metadata',[])))"`

---

## 4. Tóm tắt lệnh

| Bước | Lệnh |
|------|------|
| Chạy pipeline (nền, có DB reset) | `RUN_ASYNC=1 NAMESPACE=fortuna bash scripts/pipeline/full-clean-database-rebuild-deploy.sh --db-reset` |
| Theo dõi log | `tail -f /tmp/clean-rebuild-deploy.log` |
| Verify sau deploy | `NAMESPACE=fortuna bash scripts/verify/verify-after-deploy-and-e2e.sh` |

Sau verify, xem báo cáo trong `docs/test-results/verify-after-deploy-*.md` để đối chiếu **process** (migrations, seed khi FORTUNA_ENABLE_SEED_DATA=true) và **dữ liệu thực tế** (pods, catalog, crossChecks, E2E). Nếu pipeline bị timeout giữa chừng, chạy lại deploy: `bash scripts/deploy/deploy-fortuna-robust.sh` hoặc `kubectl apply -f deploy/fortuna-core-deployment.yaml` rồi chạy verify lại.

---

## 5. Giải thích kết quả verify

| Kiểm tra | Ý nghĩa |
|----------|---------|
| **Pods** | Core, Agent, Dashboard, Postgres, NATS Running → deploy ổn. |
| **Core logs (050/051/061)** | Thấy "Seed capability_metadata" / "Seed promotion_rules" (khi FORTUNA_ENABLE_SEED_DATA=true) → seed đã chạy (Core + DB mới hoặc --db-reset). |
| **Capability Catalog count > 0** | Bảng `capability_metadata` đã được seed → Capability Catalog có dữ liệu. Nếu = 0: FORTUNA_ENABLE_SEED_DATA chưa bật hoặc migrations 050/061 đã chạy trước khi bật env. Xem `docs/FORTUNA_ENABLE_SEED_DATA.md`. |
| **data-integrity JSON** | Khi bật auth, gọi không token có thể 401 → script verify gọi không token; nếu cần có thể gọi từ E2E (có login) hoặc dashboard. |
| **E2E pod-delete** | APIs PCE/risk trả JSON; runtime risk cho pod đã xóa → 404. Một fail: health/dashboard-data-integrity khi auth bật (E2E dùng token nhưng path có thể khác). |

**Để Capability Catalog có dữ liệu:** Cần **FORTUNA_ENABLE_SEED_DATA=true** trong Core deployment và **DB reset** (--db-reset) để migrations 050/061 chạy lại; hoặc deploy mới từ đầu. Xem `docs/FORTUNA_ENABLE_SEED_DATA.md`.
