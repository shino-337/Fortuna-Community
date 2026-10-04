# Fortuna Dashboard

React/Vite frontend for **Fortuna**. Connects to Core APIs (`/api/v1/...`) and displays SBOM analysis, threat velocity, and risk insights.

## Build Production Image (containerd only – no npm on host)

**Production build never uses npm on the host.** Dashboard is built only inside the container via the Dockerfile. You do **not** need Node.js or npm installed on your machine.

From repo root:

```bash
./scripts/build/build-and-load-containerd.sh
# or dashboard only:
BUILD_DASHBOARD_ONLY=true ./scripts/build/build-and-load-containerd.sh
```

Or with nerdctl directly:

```bash
nerdctl --namespace k8s.io build -f dashboard/Dockerfile -t fortuna-dashboard:latest .
```

Host requirements for production build: **nerdctl** and **containerd** only. `npm install` and `npm run build` run inside the image. Tailwind CSS is built via PostCSS (no CDN in production).

## Run Locally (optional – only for development)

Use **Node.js 24 LTS** for the same version used by CI and the Dashboard builder image. npm bundles **`npx`** (same version as npm).

On Ubuntu, if `apt install nodejs` from the distro is too old, use the [NodeSource installation instructions](https://github.com/nodesource/distributions) for Node 24. If `dpkg` reports `libnode-dev` file conflicts, remove the conflicting distro development package before installing Node 24.

From `dashboard/`:

1. `npm ci` (or `npm install`)
2. `npm run typecheck` — `tsc --noEmit`
3. `npm run build` — production bundle (same as Dockerfile build step)
4. (Optional) `VITE_CORE_API_URL=http://localhost:8080` then `npm run dev`

## Deploy in Kubernetes

The dashboard is deployed with the rest of Fortuna (`deploy/dashboard-deployment.yaml` plus `deploy/dashboard-nginx-configmap.yaml`); see [Install on a cluster](../docs/01-getting-started/QUICKSTART.md). It runs as the `fortuna-dashboard` Service in the `fortuna` namespace.
