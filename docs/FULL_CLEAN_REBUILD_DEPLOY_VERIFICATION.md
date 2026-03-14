# Kiểm tra Full Clean → Rebuild → Deploy

Đảm bảo mọi thay đổi code được áp dụng khi chạy script full clean rebuild deploy.

---

## 1. Luồng script (thứ tự)

| Phase | Script | Hành động |
|-------|--------|-----------|
| 1 | full-clean-database-rebuild-deploy.sh | **Clean:** port-forward kill, E2E ns delete, **xóa toàn bộ image fortuna** (theo tag + theo image ID), **system prune**, **builder prune** |
| 2 | build-and-load-containerd.sh | **Rebuild:** nerdctl build với **NO_CACHE=true** (khi vừa chạy clean), tag **fortuna-core:VERSION** + **fortuna-core:latest**, tương tự agent và dashboard. Context = PROJECT_ROOT. |
| 2b | push-images-to-workers.sh | **Push** (nếu có): đẩy core + agent lên tất cả node (master + worker). **Dashboard không được push** – nếu build chạy trên node khác với node chạy dashboard, cần build trên đúng node hoặc bổ sung push dashboard. |
| 3 | deploy-fortuna-robust.sh + apply | **Deploy:** apply infra, RBAC, core, agent, dashboard YAML. **Rollout restart** deployment core, dashboard, daemonset agent. Chờ rollout status cả 3. |

---

## 2. Clean (cache, old images)

- **Image fortuna:** Script xóa theo thứ tự:
  1. `nerdctl rmi fortuna-core:latest fortuna-agent:latest fortuna-dashboard:latest`
  2. `nerdctl images | grep fortuna-(core|agent|dashboard)` → lấy cột image ID → `nerdctl rmi <id>` từng cái
  3. `nerdctl system prune -f`
  4. `nerdctl builder prune -a -f`
- **Kết quả:** Không còn image fortuna, không còn build cache → lần build sau dùng 100% source hiện tại (và NO_CACHE).

---

## 3. Build: tag và context

- **Build script:** `build-and-load-containerd.sh`
  - VERSION = `git describe --tags --always --dirty` hoặc `latest`
  - Tag mỗi image: `fortuna-<component>:${VERSION}` và `fortuna-<component>:latest`
  - Context: `.` (PROJECT_ROOT)
  - **NO_CACHE:** Khi gọi từ full script sau khi chạy clean, script set `NO_CACHE=true` và truyền xuống build → `nerdctl build --no-cache ...`
- **Dockerfile và context:**
  - **core/Dockerfile:** COPY api/, COPY core/ → context phải là repo root. Gọi: `nerdctl build -f core/Dockerfile -t fortuna-core:latest .` ✓
  - **agent/Dockerfile:** COPY api/, COPY agent/ → context repo root. Gọi: `nerdctl build -f agent/Dockerfile -t fortuna-agent:latest .` ✓
  - **dashboard/Dockerfile:** COPY dashboard/ → context repo root. Gọi: `nerdctl build -f dashboard/Dockerfile -t fortuna-dashboard:latest .` ✓

---

## 4. Deployment YAML (image và imagePullPolicy)

| File | Image | imagePullPolicy |
|------|--------|------------------|
| deploy/fortuna-core-deployment.yaml | fortuna-core:latest | Never |
| deploy/fortuna-agent-daemonset.yaml | fortuna-agent:latest | IfNotPresent |
| deploy/dashboard-deployment.yaml | fortuna-dashboard:latest | Never |

- **Never:** Node phải có sẵn image (build tại chỗ hoặc push từ máy build). Sau clean + rebuild, image :latest trên máy build là bản mới; nếu deploy chạy trên cùng máy thì pod dùng đúng bản mới.
- **IfNotPresent:** Có thể dùng cache node; sau khi push (hoặc build local) image mới, rollout restart sẽ tạo pod mới và kubelet lấy image :latest (bản vừa build/push).

---

## 5. Kiểm tra nhanh sau khi chạy script

1. **Image trên node (nerdctl/ctr):**
   ```bash
   nerdctl --namespace k8s.io images | grep fortuna
   ```
   Phải thấy fortuna-core, fortuna-agent, fortuna-dashboard với tag latest và CREATED vừa xong.

2. **Pod đang chạy bản mới:**
   - Core: `kubectl get pods -n fortuna -l app.kubernetes.io/component=core -o wide` → AGE vài phút (sau rollout).
   - Dashboard: `kubectl exec -n fortuna deployment/fortuna-dashboard -- cat /usr/share/nginx/html/version.txt` → có file và build time mới.
   - Agent: `kubectl get pods -n fortuna -l app=fortuna-agent` → AGE vài phút sau rollout.

3. **Rollout:**
   ```bash
   ./scripts/pipeline/verify-rollout.sh
   ```

---

## 6. Lưu ý

- **Dashboard không có trong push-images-to-workers.sh:** Chỉ core và agent được push. Nếu build chạy trên máy A, cluster trên máy B, thì dashboard phải được build/copy lên đúng node có pod dashboard (vd. control-plane), hoặc cần bổ sung push dashboard vào script.
- **NO_CACHE:** Chỉ bật khi vừa chạy Phase 1 (clean). Nếu chỉ chạy `--skip-clean` rồi rebuild, có thể vẫn dùng cache; muốn build sạch thì chạy full (clean + rebuild) hoặc gọi build với `NO_CACHE=true`.
