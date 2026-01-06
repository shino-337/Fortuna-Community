# Fortuna Dashboard

**Version**: 1.0.0  
**Status**: In Development

Modern web dashboard for the Fortuna security platform. Provides a comprehensive UI for viewing security insights, SBOMs, CVEs, risk scores, and cluster resources.

---

## Overview

Fortuna Dashboard is a React-based single-page application that connects to Fortuna Core's REST API to provide a user-friendly interface for security analysis and cluster management.

### Key Features

- **Security Insights**: View and manage security insights with filtering and actions
- **SBOM Browser**: Explore Software Bill of Materials for containers
- **CVE Management**: View CVE matches and vulnerability details
- **Risk Analytics**: Risk scores, trends, and analytics
- **Cluster Management**: Multi-cluster overview and management
- **Resource Browser**: Explore pods, service accounts, and other resources
- **Attack Path Visualization**: Graph-based attack path analysis (Coming soon)
- **Real-Time Updates**: Live updates via API polling

---

## Tech Stack

- **React 18** + **TypeScript**: Modern UI framework
- **Vite**: Fast build tool and dev server
- **Tailwind CSS**: Utility-first CSS framework
- **React Router**: Client-side routing
- **Zustand**: Lightweight state management
- **React Query**: Data fetching and caching
- **Axios**: HTTP client
- **Recharts**: Chart library for analytics

---

## Getting Started

### Prerequisites

- Node.js 18+
- npm or yarn
- Access to Fortuna Core API

### Installation

```bash
npm install
```

### Development

1. **Set Environment Variables**:

Create `.env` file:
```env
VITE_API_URL=http://localhost:8080
```

2. **Start Dev Server**:

```bash
npm run dev
```

Dashboard will be available at `http://localhost:5173`

### Build

```bash
npm run build
```

Production build will be in `dist/` directory.

### Preview Production Build

```bash
npm run preview
```

---

## Project Structure

```
dashboard/
├── src/
│   ├── components/          # Reusable components
│   │   ├── ui/             # UI components (Button, Card, Table, etc.)
│   │   ├── Layout.tsx      # Main layout with sidebar
│   │   ├── StatCard.tsx    # Statistics card
│   │   └── ...
│   ├── pages/              # Page components
│   │   ├── Login.tsx       # Login page
│   │   ├── Dashboard.tsx   # Main dashboard
│   │   ├── Insights.tsx    # Security insights
│   │   ├── SBOMs.tsx       # SBOM browser
│   │   ├── CVEs.tsx        # CVE management
│   │   ├── Risk.tsx        # Risk analytics
│   │   ├── Clusters.tsx    # Cluster management
│   │   ├── Pods.tsx        # Pod browser
│   │   └── ...
│   ├── lib/                # Utilities
│   │   ├── api.ts          # API client (Axios)
│   │   ├── auth.ts         # Authentication utilities
│   │   └── utils.ts        # Helper functions
│   ├── store/              # State management (Zustand)
│   │   ├── authStore.ts    # Authentication state
│   │   └── ...
│   ├── types/              # TypeScript types
│   │   ├── api.ts          # API response types
│   │   └── ...
│   ├── hooks/              # Custom React hooks
│   └── App.tsx             # Root component
├── public/                  # Static assets
├── package.json
├── vite.config.ts
├── tailwind.config.js
└── tsconfig.json
```

---

## API Integration

### Base URL

The dashboard connects to Fortuna Core API. Default configuration:

- **Development**: `http://localhost:8080`
- **Production**: `http://fortuna-core.fortuna.svc.cluster.local:8080`

Configure via `VITE_API_URL` environment variable.

### Available Endpoints

The dashboard uses the following Fortuna Core API endpoints:

#### Authentication
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/logout` - User logout

#### Insights
- `GET /api/v1/insights` - List insights (with filters)
- `GET /api/v1/insights/summary` - Insights summary
- `GET /api/v1/insights/:id` - Get insight details
- `POST /api/v1/insights/:id/resolve` - Resolve insight
- `POST /api/v1/insights/:id/dismiss` - Dismiss insight

#### SBOMs
- `GET /api/v1/sboms` - List SBOMs
- `GET /api/v1/sboms/:id` - Get SBOM details
- `GET /api/v1/sboms/:id/components` - Get SBOM components

#### CVEs
- `GET /api/v1/cves` - List CVEs
- `GET /api/v1/cves/:id` - Get CVE details
- `GET /api/v1/cves/:id/matches` - Get CVE matches

#### Risk
- `GET /api/v1/risk/scores` - Risk scores
- `GET /api/v1/risk/trends` - Risk trends
- `GET /api/v1/risk/analytics` - Risk analytics

#### Clusters
- `GET /api/v1/clusters` - List clusters
- `GET /api/v1/clusters/:id` - Get cluster details

#### Pods
- `GET /api/v1/pods` - List pods
- `GET /api/v1/pods/:id` - Get pod details

#### Graph
- `GET /api/v1/graph/blast-radius` - Blast radius analysis
- `GET /api/v1/graph/attack-paths` - Attack path visualization

See [API Reference](../../docs/API_REFERENCE.md) for complete API documentation.

---

## Features

### ✅ Implemented

- [x] Authentication (Login/Logout)
- [x] Dashboard Overview
- [x] Security Insights (List, Filter, Actions)
- [x] Insights Summary
- [x] SBOM Browser
- [x] CVE Management
- [x] Risk Analytics
- [x] Cluster Management
- [x] Pod Browser
- [x] Service Account Browser
- [x] Settings

### 🚧 In Progress

- [ ] Attack Path Visualization
- [ ] Real-time WebSocket updates
- [ ] Advanced filtering
- [ ] Export functionality

### 📋 Planned

- [ ] Policy management UI
- [ ] Audit log viewer
- [ ] Custom dashboards
- [ ] Alerting configuration

---

## Deployment

### Docker

```dockerfile
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

Build and run:
```bash
docker build -t fortuna-dashboard:latest .
docker run -p 8080:80 fortuna-dashboard:latest
```

### Kubernetes

See `deploy/dashboard-deployment.yaml` for Kubernetes deployment manifest.

**Note**: Dashboard deployment is optional and not included in the main production deployment. It can be deployed separately when needed.

---

## Development

### Code Style

- Use TypeScript for type safety
- Follow React best practices
- Use functional components with hooks
- Prefer composition over inheritance

### State Management

- **Zustand**: Global state (auth, user preferences)
- **React Query**: Server state (API data)
- **Local State**: Component-specific state

### API Client

The API client (`lib/api.ts`) uses Axios with:
- Automatic token injection
- Request/response interceptors
- Error handling
- TypeScript types

### Styling

- **Tailwind CSS**: Utility-first CSS
- **Responsive Design**: Mobile-first approach
- **Dark Mode**: Coming soon

---

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_URL` | Fortuna Core API URL | `http://localhost:8080` |
| `VITE_AUTH_ENABLED` | Enable authentication | `true` |

---

## Troubleshooting

### API Connection Issues

- Verify `VITE_API_URL` is correct
- Check CORS configuration in Core
- Verify network connectivity

### Authentication Issues

- Check JWT token expiration
- Verify `AUTH_ENABLED` in Core
- Check token storage (localStorage)

### Build Issues

- Clear `node_modules` and reinstall
- Check Node.js version (18+)
- Verify TypeScript version

---

## Related Documentation

- [API Reference](../../docs/API_REFERENCE.md)
- [Architecture](../../docs/ARCHITECTURE.md)
- [Core README](../core/README.md)

---

**Version**: 1.0.0  
**Last Updated**: 2026-01-06  
**Status**: In Development
