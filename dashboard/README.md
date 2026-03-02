
# Fortuna Dashboard

React/Vite frontend for Fortuna. It connects to Core APIs (`/api/v1/...`) and displays
SBOM analysis, threat velocity, and risk insights.

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

Only if you want to develop or run the app on the host (requires Node.js 18+ and npm):

1. `npm install`
2. (Optional) `VITE_CORE_API_URL=http://localhost:8080`
3. `npm run dev`

## Deploy in Kubernetes

Apply the manifest:

```
kubectl apply -f deploy/dashboard-deployment.yaml
```

Dashboard runs as `fortuna-dashboard` service in the `fortuna` namespace.
