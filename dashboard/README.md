# KSAM Dashboard

Web UI cho Kubernetes Service Account Manager với graph visualization.

## Chức năng

- Hiển thị interactive graph về relationships giữa ServiceAccounts, Roles, Namespaces
- Filtering theo cluster, namespace, role, privilege level
- Risk scoring và visualization
- Time-based views của SA changes
- Audit reports và compliance dashboard

## Cấu trúc

```
dashboard/
├── src/
│   ├── components/       # React components
│   │   ├── Graph/        # Graph visualization
│   │   ├── ServiceAccountList/
│   │   ├── ClusterList/
│   │   └── AuditLogs/
│   ├── pages/            # Page components
│   ├── hooks/            # Custom React hooks
│   ├── services/         # API clients
│   ├── types/            # TypeScript types
│   ├── utils/            # Utility functions
│   └── App.tsx           # Main app component
├── public/
├── package.json
├── tsconfig.json
├── vite.config.ts
└── tailwind.config.js
```

## Development

```bash
npm install
npm run dev
```

## Build

```bash
npm run build
```

## Tech Stack

- React 18
- TypeScript
- Vite
- Tailwind CSS
- Cytoscape.js (graph visualization)
- React Query (data fetching)

