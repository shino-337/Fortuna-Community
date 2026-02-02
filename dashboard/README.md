<div align="center">
<img width="1200" height="475" alt="GHBanner" src="https://github.com/user-attachments/assets/0aa67016-6eaf-458a-adb2-6e31a0763ed6" />
</div>

# Fortuna Dashboard

React/Vite frontend for Fortuna. It connects to Core APIs (`/api/v1/...`) and displays
SBOM analysis, threat velocity, and risk insights.

## Run Locally

**Prerequisites:** Node.js 18+

1. Install dependencies:
   `npm install`
2. (Optional) Set Core API URL:
   - `VITE_CORE_API_URL=http://localhost:8080`
   - If not set, the app will call `/api/v1` relative to the current host.
3. Run the app:
   `npm run dev`

## Build Production Assets

`npm run build`

## Docker Build

From repo root:

```
nerdctl --namespace k8s.io build -f dashboard/Dockerfile -t fortuna-dashboard:latest .
```

The Docker image uses `nginx.conf` to proxy `/api/*` to:
`http://fortuna-core.fortuna.svc.cluster.local:8080`.

## Deploy in Kubernetes

Apply the manifest:

```
kubectl apply -f deploy/dashboard-deployment.yaml
```

Dashboard runs as `fortuna-dashboard` service in the `fortuna` namespace.
