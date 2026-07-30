# Fortuna Dashboard

React/Vite frontend for **FortunaK8s** (K8S Security & Risk Management Platform). Connects to Core APIs (`/api/v1/...`) and displays SBOM analysis, threat velocity, and risk insights.

## Build Production Image (containerd only – no npm on host)

**Production build never uses npm on the host.** Dashboard is built only inside the container via the Dockerfile. You do **not** need Node.js or npm installed on your machine.

From repo root:

```bash
./scripts/build/build-and-load-containerd.sh
# or dashboard only:
./scripts/build/build-dashboard-containerd.sh
```

Or with nerdctl directly:

```bash
nerdctl --namespace k8s.io build -f dashboard/Dockerfile -t fortuna-dashboard:latest .
```

Host requirements for production build: **nerdctl** and **containerd** only. `npm install` and `npm run build` run inside the image. Tailwind CSS is built via PostCSS (no CDN in production).

## Run Locally (optional – only for development)

Use **Node.js 20+** (Vite 6 / React Router 7 declare `^20.19` or `>=22.12`). npm bundles **`npx`** (same version as npm).

On Ubuntu, if `apt install nodejs` from distro is too old, use [NodeSource Node 20](https://github.com/nodesource/distributions). If `dpkg` fails with `libnode-dev` file conflicts, remove the distro meta-package first: `apt remove -y libnode-dev` (or `apt install nodejs` from NodeSource after removing conflicting `-dev` packages), then install Node 20.

From `dashboard/`:

1. `npm ci` (or `npm install`)
2. `npm run typecheck` — `tsc --noEmit`
3. `npm run build` — production bundle (same as Dockerfile build step)
4. (Optional) `VITE_CORE_API_URL=http://localhost:8080` then `npm run dev`

## Deploy in Kubernetes

Apply the manifest:

```
kubectl apply -f deploy/dashboard-deployment.yaml
```

Dashboard runs as `fortuna-dashboard` service in the `fortuna` namespace.
