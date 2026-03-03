# FortunaK8s

**K8S Security & Risk Management Platform** – SBOM extraction, CVE matching, security insights, Pod Capability Engine (PCE), runtime signals, and web dashboard.

---

## Quick links

| Resource | Description |
|----------|-------------|
| **[docs/README.md](docs/README.md)** | Full documentation index (getting started, architecture, operations, components, testing) |
| **[docs-prod/README.md](docs-prod/README.md)** | Production-focused docs (overview, architecture, user guide, operations, configuration) |
| **[deploy/README.md](deploy/README.md)** | Deploy checklist, manifests, Helm, troubleshooting |
| **[scripts/README.md](scripts/README.md)** | All scripts (pipeline, deploy, verify, E2E, monitor) |

---

## Quick start

```bash
# 1. Build images (nerdctl → containerd)
./scripts/build/build-and-load-containerd.sh

# 2. Deploy (infra + mTLS + Core + Agent + Dashboard)
./scripts/deploy/deploy-fortuna-robust.sh

# 3. Verify
./scripts/verify/check-full-deployment.sh
```

**Full pipeline (clean + rebuild + deploy):**

```bash
./scripts/pipeline/full-clean-database-rebuild-deploy.sh
# Optional: --db (clear DB data) or --db-reset (full schema reset)
```

**Access:**  
Dashboard: `kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80` → http://localhost:8081  
Core API: `kubectl port-forward -n fortuna svc/fortuna-core 8080:8080` → http://localhost:8080/health  

Default login: `admin` / `admin123` (change in production).

---

## Repository structure

| Path | Contents |
|------|----------|
| **core/** | Core service (Go): REST/gRPC API, CVE matching, PCE, migrations |
| **agent/** | Agent DaemonSet (Go): pod watch, SBOM extraction, sync to Core |
| **dashboard/** | Web UI (React/Vite): Risk Center, SBOM, PCE, runtime signals |
| **deploy/** | Kubernetes manifests (Core, Agent, Dashboard, infra, RBAC, mTLS) |
| **docs/** | Documentation (getting started, architecture, operations, components, reference) |
| **docs-prod/** | Production docs (overview, architecture, features, user guide, operations, configuration) |
| **scripts/** | Pipeline, deploy, clean, build, verify, E2E, monitor (see [scripts/README.md](scripts/README.md)) |
| **script-prod/** | Production build/deploy/verify/clean (versioned images, config-driven) |

---

## Main components

- **Core** – Central API, DB (PostgreSQL), NATS, migrations, PCE scheduler.
- **Agent** – One pod per node; pod sync, SBOM extraction, gRPC (mTLS) to Core.
- **Dashboard** – React UI; proxies `/api` to Core; Risk Center, SBOM, pod detail, capabilities.

---

## License

See repository root for license information.

---

*Last updated: 2026-03-03*
