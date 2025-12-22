# KSAM Dashboard

Modern web dashboard for Fortuna K8s Management Platform.

## Tech Stack

- **React 18** + **TypeScript**
- **Vite** - Build tool
- **Tailwind CSS** - Styling
- **React Router** - Routing
- **Zustand** - State management
- **React Query** - Data fetching
- **Axios** - HTTP client

## Getting Started

### Prerequisites

- Node.js 18+
- npm or yarn

### Installation

```bash
npm install
```

### Development

```bash
npm run dev
```

Dashboard will be available at `http://localhost:5173`

### Build

```bash
npm run build
```

### Environment Variables

Create `.env` file:

```
VITE_API_URL=http://localhost:8080
```

## Features

- ✅ Authentication (Login/Logout)
- ✅ Dashboard Overview
- ✅ Clusters Management
- ✅ Security Insights
- ✅ Resources Browser
- ✅ Attack Path Visualization (Coming soon)
- ✅ System Metrics (Coming soon)
- ✅ Settings

## Project Structure

```
dashboard/
├── src/
│   ├── components/      # Reusable components
│   │   ├── ui/         # UI components (Button, Card, etc.)
│   │   ├── Layout.tsx  # Main layout with sidebar
│   │   └── StatCard.tsx
│   ├── pages/          # Page components
│   │   ├── Login.tsx
│   │   ├── Dashboard.tsx
│   │   ├── Clusters.tsx
│   │   ├── Insights.tsx
│   │   └── ...
│   ├── lib/            # Utilities
│   │   ├── api.ts      # API client
│   │   └── auth.ts     # Auth utilities
│   ├── store/          # State management
│   │   └── authStore.ts
│   ├── types/          # TypeScript types
│   └── App.tsx
└── package.json
```

## API Integration

The dashboard connects to KSAM Core API at `http://localhost:8080` by default.

### Available Endpoints

- `GET /api/v1/clusters` - List clusters
- `GET /api/v1/insights` - List insights
- `GET /api/v1/insights/summary` - Insights summary
- `GET /api/v1/pods` - List pods
- `POST /api/v1/auth/login` - Login

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
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### Kubernetes

See `KSAM/deploy/dashboard-deployment.yaml` for Kubernetes deployment manifest.

## Development Status

- ✅ Phase 1: Foundation (Complete)
- ✅ Phase 2: Core Screens (In Progress)
- ⏳ Phase 3: Advanced Features (Pending)
- ⏳ Phase 4: Polish & Testing (Pending)
