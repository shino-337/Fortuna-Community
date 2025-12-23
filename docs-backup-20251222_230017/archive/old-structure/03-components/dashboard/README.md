# Dashboard Component

The KSAM Dashboard provides a modern React-based web interface for visualizing security insights, managing policies, and analyzing risks.

---

## Overview

**Status**: ✅ Production Ready (MVP2)
**Technology**: React + TypeScript + Vite
**UI Library**: Tailwind CSS + shadcn/ui
**State Management**: TanStack Query (React Query)
**Charts**: Recharts

---

## Features

### Implemented (MVP2)

✅ **Authentication**
- JWT-based login
- Protected routes
- Session management
- Auto-refresh tokens

✅ **Dashboard Overview**
- Real-time metrics
- Risk score trends
- Top insights
- Recent violations

✅ **Risk Center**
- Active insights list
- Severity filtering
- Status management (active/resolved/dismissed)
- Insight details modal

✅ **ServiceAccounts View**
- List all ServiceAccounts
- Risk score display
- Associated pods count
- Role bindings count

✅ **Pods View**
- Pod inventory
- Image information
- ServiceAccount association
- Namespace filtering

✅ **Attack Paths** (Visualization)
- D3.js force-directed graph
- Interactive zoom/pan
- Node details on hover
- Path highlighting

✅ **Metrics Dashboard**
- System metrics
- Worker statistics
- API performance
- Database metrics

---

## Architecture

```
┌──────────────────────────────────────────┐
│         React Frontend (Vite)            │
│                                          │
│  ┌────────┐  ┌────────┐  ┌──────────┐  │
│  │ Login  │  │Dashboard│  │Risk Ctr  │  │
│  └────────┘  └────────┘  └──────────┘  │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │    TanStack Query (Data Fetching)  │ │
│  └─────────────┬──────────────────────┘ │
└────────────────┼─────────────────────────┘
                 │ HTTP/REST
                 ▼
┌────────────────────────────────────────┐
│       KSAM Core API (Port 8080)        │
│                                        │
│  GET  /api/v1/insights                 │
│  GET  /api/v1/serviceaccounts          │
│  GET  /api/v1/pods                     │
│  GET  /api/v1/risk-trends              │
│  GET  /api/v1/graph/attack-paths       │
│  POST /api/v1/login                    │
└────────────────────────────────────────┘
```

---

## Quick Start

### Development

```bash
cd dashboard

# Install dependencies
npm install

# Set API endpoint
echo "VITE_API_URL=http://localhost:8080" > .env.local

# Start dev server
npm run dev

# Open http://localhost:5173
```

### Production Build

```bash
# Build for production
npm run build

# Output: dashboard/dist/

# Preview production build
npm run preview
```

### Docker Deployment

```bash
# Build image
docker build -t ksam/dashboard:latest dashboard/

# Run container
docker run -p 80:80 \
  -e API_URL=http://ksam-core:8080 \
  ksam/dashboard:latest
```

---

## Configuration

### Environment Variables

```bash
# .env.local (development)
VITE_API_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws
VITE_AUTH_ENABLED=true

# Production (via nginx config)
API_URL=http://ksam-core:8080
```

### nginx Configuration

```nginx
# dashboard/nginx.conf
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    # SPA routing
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy API requests to Core
    location /api/ {
        proxy_pass ${API_URL};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        # CORS
        add_header Access-Control-Allow-Origin *;
        add_header Access-Control-Allow-Methods "GET, POST, PUT, DELETE, OPTIONS";
        add_header Access-Control-Allow-Headers "Authorization, Content-Type";

        if ($request_method = 'OPTIONS') {
            return 204;
        }
    }

    # WebSocket support
    location /ws {
        proxy_pass ${API_URL};
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## Pages & Routes

### Public Routes

```tsx
// src/App.tsx
<Routes>
  <Route path="/login" element={<Login />} />
</Routes>
```

### Protected Routes

```tsx
// src/App.tsx (wrapped in <ProtectedRoute>)
<Routes>
  <Route path="/" element={<Dashboard />} />
  <Route path="/risk-center" element={<RiskCenter />} />
  <Route path="/serviceaccounts" element={<ServiceAccounts />} />
  <Route path="/pods" element={<Pods />} />
  <Route path="/attack-paths" element={<AttackPaths />} />
  <Route path="/metrics" element={<Metrics />} />
  <Route path="/settings" element={<Settings />} />
</Routes>
```

---

## Components

### Layout Components

**`src/components/Layout.tsx`**:
```tsx
export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-auto">
        <Header />
        <div className="p-6">{children}</div>
      </main>
    </div>
  );
}
```

### Data Fetching (TanStack Query)

**`src/pages/RiskCenter.tsx`**:
```tsx
import { useQuery } from '@tanstack/react-query';

export function RiskCenter() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['insights', { severity: 'CRITICAL', status: 'active' }],
    queryFn: () =>
      fetch('http://localhost:8080/api/v1/insights?severity=CRITICAL&status=active')
        .then(res => res.json()),
    refetchInterval: 30000, // Auto-refresh every 30s
  });

  if (isLoading) return <LoadingSpinner />;
  if (error) return <ErrorMessage error={error} />;

  return (
    <div>
      <h1>Risk Center</h1>
      <InsightsList insights={data.insights} />
    </div>
  );
}
```

### Charts (Recharts)

**`src/components/RiskTrendChart.tsx`**:
```tsx
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip } from 'recharts';

export function RiskTrendChart({ data }: { data: RiskTrend[] }) {
  return (
    <LineChart width={600} height={300} data={data}>
      <CartesianGrid strokeDasharray="3 3" />
      <XAxis dataKey="date" />
      <YAxis />
      <Tooltip />
      <Line type="monotone" dataKey="avg_risk_score" stroke="#8884d8" />
      <Line type="monotone" dataKey="critical_count" stroke="#ff0000" />
    </LineChart>
  );
}
```

### Attack Path Visualization (D3.js)

**`src/components/AttackPathGraph.tsx`**:
```tsx
import * as d3 from 'd3';

export function AttackPathGraph({ paths }: { paths: AttackPath[] }) {
  useEffect(() => {
    const svg = d3.select('#graph-svg');
    const simulation = d3.forceSimulation(nodes)
      .force('link', d3.forceLink(links))
      .force('charge', d3.forceManyBody().strength(-400))
      .force('center', d3.forceCenter(width / 2, height / 2));

    // Render nodes and edges
    // ...
  }, [paths]);

  return <svg id="graph-svg" width={800} height={600} />;
}
```

---

## API Integration

### Authentication

```tsx
// src/lib/api.ts
export async function login(username: string, password: string) {
  const response = await fetch('http://localhost:8080/api/v1/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });

  const { token } = await response.json();
  localStorage.setItem('token', token);
  return token;
}

export function getAuthHeaders() {
  const token = localStorage.getItem('token');
  return {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json',
  };
}
```

### API Client

```tsx
// src/lib/api.ts
export const api = {
  insights: {
    list: (params?: InsightParams) =>
      fetch(`/api/v1/insights?${new URLSearchParams(params)}`, {
        headers: getAuthHeaders(),
      }).then(res => res.json()),

    get: (id: string) =>
      fetch(`/api/v1/insights/${id}`, {
        headers: getAuthHeaders(),
      }).then(res => res.json()),

    resolve: (id: string, reason: string) =>
      fetch(`/api/v1/insights/${id}/resolve`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({ reason }),
      }).then(res => res.json()),
  },

  serviceAccounts: {
    list: () =>
      fetch('/api/v1/serviceaccounts', {
        headers: getAuthHeaders(),
      }).then(res => res.json()),
  },

  // ... other endpoints
};
```

---

## Deployment

### Kubernetes

**`deploy/dashboard-deployment.yaml`**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ksam-dashboard
  namespace: ksam
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ksam-dashboard
  template:
    metadata:
      labels:
        app: ksam-dashboard
    spec:
      containers:
      - name: dashboard
        image: ksam/dashboard:latest
        ports:
        - containerPort: 80
        env:
        - name: API_URL
          value: "http://ksam-core:8080"
---
apiVersion: v1
kind: Service
metadata:
  name: ksam-dashboard
  namespace: ksam
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: ksam-dashboard
```

### Access Dashboard

```bash
# Get external IP
kubectl get svc -n ksam ksam-dashboard

# Port-forward for local access
kubectl port-forward -n ksam svc/ksam-dashboard 8081:80

# Open browser
open http://localhost:8081
```

---

## Troubleshooting

### Dashboard Not Loading

**Check nginx**:
```bash
kubectl logs -n ksam ksam-dashboard-* | grep nginx
```

**Check deployment**:
```bash
kubectl get pods -n ksam -l app=ksam-dashboard
kubectl describe pod -n ksam ksam-dashboard-*
```

### API Requests Failing (CORS)

**Symptom**: Browser console shows CORS errors

**Fix**:
```nginx
# Add to nginx.conf
add_header Access-Control-Allow-Origin *;
add_header Access-Control-Allow-Methods "GET, POST, PUT, DELETE, OPTIONS";
add_header Access-Control-Allow-Headers "Authorization, Content-Type";

if ($request_method = 'OPTIONS') {
    return 204;
}
```

**Rebuild**:
```bash
docker build -t ksam/dashboard:latest dashboard/
kubectl rollout restart deployment/ksam-dashboard -n ksam
```

### Authentication Not Working

**Check token**:
```tsx
// In browser console
localStorage.getItem('token')
```

**Test login**:
```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'
```

### Data Not Updating

**Check auto-refresh**:
```tsx
// Ensure refetchInterval is set
useQuery({
  queryKey: ['insights'],
  queryFn: fetchInsights,
  refetchInterval: 30000, // 30 seconds
});
```

---

## Development Guide

### Project Structure

```
dashboard/
├── src/
│   ├── components/          # Reusable UI components
│   │   ├── Layout.tsx
│   │   ├── StatCard.tsx
│   │   └── ui/             # shadcn/ui components
│   ├── pages/              # Route pages
│   │   ├── Dashboard.tsx
│   │   ├── RiskCenter.tsx
│   │   └── Login.tsx
│   ├── lib/                # Utilities
│   │   ├── api.ts          # API client
│   │   └── utils.ts
│   ├── types/              # TypeScript types
│   │   └── index.ts
│   ├── store/              # State management (if needed)
│   ├── App.tsx             # Main app component
│   └── main.tsx            # Entry point
├── public/                 # Static assets
├── index.html
├── package.json
├── vite.config.ts
├── tailwind.config.js
└── tsconfig.json
```

### Adding a New Page

1. **Create page component**:
```tsx
// src/pages/NewFeature.tsx
export function NewFeature() {
  const { data } = useQuery({
    queryKey: ['newFeature'],
    queryFn: () => api.newFeature.list(),
  });

  return (
    <Layout>
      <h1>New Feature</h1>
      {/* ... */}
    </Layout>
  );
}
```

2. **Add route**:
```tsx
// src/App.tsx
<Route path="/new-feature" element={<NewFeature />} />
```

3. **Add navigation**:
```tsx
// src/components/Layout.tsx (Sidebar)
<Link to="/new-feature">New Feature</Link>
```

### Styling Guidelines

**Use Tailwind**:
```tsx
<div className="flex items-center justify-between p-4 bg-gray-100 rounded-lg">
  <h2 className="text-xl font-bold text-gray-900">Title</h2>
</div>
```

**Use shadcn/ui**:
```tsx
import { Button } from '@/components/ui/button';
import { Card, CardHeader, CardContent } from '@/components/ui/card';

<Card>
  <CardHeader>Title</CardHeader>
  <CardContent>
    <Button variant="default">Click Me</Button>
  </CardContent>
</Card>
```

---

## Performance Optimization

### Code Splitting

```tsx
// Lazy load pages
const RiskCenter = lazy(() => import('./pages/RiskCenter'));
const AttackPaths = lazy(() => import('./pages/AttackPaths'));

<Suspense fallback={<LoadingSpinner />}>
  <Route path="/risk-center" element={<RiskCenter />} />
</Suspense>
```

### Query Optimization

```tsx
// Prefetch on hover
const queryClient = useQueryClient();

<Link
  to="/risk-center"
  onMouseEnter={() => {
    queryClient.prefetchQuery({
      queryKey: ['insights'],
      queryFn: fetchInsights,
    });
  }}
>
  Risk Center
</Link>
```

---

## Testing

### Unit Tests

```bash
npm run test
```

### E2E Tests (Playwright)

```bash
npx playwright test
```

---

## Related Components

- [Core API](../core/) - Backend API consumed by dashboard
- [Risk Engine](../risk-engine/) - Data source for insights
- [Graph Engine](../graph-engine/) - Attack path visualization data

---

**Last Updated**: December 16, 2025
**Status**: ✅ Production Ready
**Technology Stack**: React + TypeScript + Vite + Tailwind + shadcn/ui
