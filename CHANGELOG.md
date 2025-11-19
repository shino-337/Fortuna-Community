# Changelog

All notable changes to the K8s Workload Management Platform (KSAM) will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [1.0.2] - 2024-11-19

### 🐛 Fixed
- **Critical:** Namespace filter on Graph View now works correctly
  - Root cause: Debounce timer was firing after suggestion selection
  - Fixed timer cleanup in `selectNamespace()` and `handleClearFilters()`
  - Fixed React Query cache invalidation with explicit `queryKey`
- Removed all debug logs from production build

### 🔧 Changed
- `GraphFilters.tsx`: Clear debounce timer before applying filter to prevent race conditions
- `useGraph.ts`: Use explicit `cluster` and `namespace` in `queryKey` for better cache invalidation
- React Query: Added `placeholderData: undefined` to ensure fresh data on filter changes

### 📝 Documentation
- Added `DEBUG_FILTER_ISSUE.md` documenting the debug process
- Added `RELEASE_NOTES_v1.0.2.md` with detailed testing results
- Updated `TESTING_v1.0.1.md` with comprehensive filter testing scenarios

### ✅ Tested
- Filter by `default` namespace ✅
- Filter by `kube-system` namespace ✅
- Clear filters ✅
- Switch between multiple namespaces ✅
- No duplicate API calls ✅
- No console debug logs ✅

---

## [1.0.1] - 2024-11-19

### 🎨 Added
- **Dark mode** across all pages (Dashboard, Graph View, ServiceAccounts, Audit Logs, Login)
- **RBAC guards** for admin and user roles (`RBACGuard`, `RBACButton` components)
- **New branding**: Changed from "KSAM" to "K8s Workload Management Platform"
- **Favicon**: Added custom SVG icon (`/icon.svg`)
- **Theme Context**: Implemented `ThemeContext` for global dark mode state management

### 🐛 Fixed
- Graph View dark mode issues:
  - Cytoscape container background now transparent
  - Parent container handles dark mode background
- Graph View layout issues:
  - Fixed container height (was 0)
  - Fixed Cytoscape initialization race condition using callback ref pattern
  - Legend/filters sidebar now scrolls independently
- `Symbol.toStringTag is read-only` error in Graph View filters
  - Fixed by safely copying edge data without Symbol properties
- Removed debug logs from browser console in production builds
  - Configured Vite/esbuild to `drop: ['console', 'debugger']`

### 🔧 Changed
- `GraphVisualization.tsx`: Refactored Cytoscape initialization to use callback ref pattern
- `GraphFilters.tsx`: Made filters panel responsive with independent scrolling
- `vite.config.ts`: Configured production builds to remove console logs
- Tailwind CSS: Added `darkMode: 'class'` configuration
- All UI components: Applied dark mode classes (`dark:bg-*`, `dark:text-*`)

### 📝 Documentation
- Added `UI_IMPROVEMENTS.md` documenting all UI/UX changes
- Updated `README.md` with new branding

---

## [1.0.0] - 2024-11-18

### 🎉 Initial Release

#### Features

##### Dashboard
- Overview page with cluster statistics
- ServiceAccount count, namespace count, audit log summary
- Quick actions panel

##### Graph View
- Interactive Cytoscape.js graph visualization
- Cluster and Namespace filters
- Node type filters (ServiceAccount, Namespace, Cluster)
- Connection type filters (binding, member, owner, reference)
- Multiple layout algorithms (cose, fcose, dagre, breadthfirst, circle, concentric, grid)
- Node details panel with expandable information
- Search functionality
- Collapsible sidebar
- Auto-fit and manual zoom controls

##### ServiceAccounts Management
- List all ServiceAccounts across clusters
- Filter by cluster and namespace
- View associated Roles and ClusterRoles
- Delete ServiceAccounts (with confirmation)
- Pagination support

##### Audit Logs
- Real-time audit log viewing
- Filter by resource type, action, cluster
- Auto-refresh every 10 seconds
- Pagination
- Default filter: ServiceAccount logs only

##### Authentication & Authorization
- JWT-based authentication
- Admin and User roles
- Protected routes
- Login/Logout functionality

#### Backend Components

##### Agent (Go)
- Runs as DaemonSet on each cluster node
- Collects Kubernetes resources:
  - ServiceAccounts
  - Roles / ClusterRoles
  - RoleBindings / ClusterRoleBindings
  - Pods
- **Delta Sync**: Only sends changed resources (every sync)
- **Full Sync**: Sends all resources (every 10th sync)
- **Watcher**: Real-time event monitoring for RBAC resources
- gRPC communication with Core Controller
- Configurable sync interval (default: 60s)

##### Core Controller (Go)
- Central API server
- PostgreSQL database for persistence
- Handles agent sync requests via gRPC
- RESTful API for Dashboard
- **Batch Processing**: Inserts audit logs in batches for performance
- **TTL Management**: Auto-deletes audit logs older than 90 days
- Ensures system user exists for audit logs
- Supports cluster management

##### Database
- PostgreSQL with optimized schema
- Tables:
  - `clusters`: Cluster information
  - `service_accounts`: ServiceAccount data
  - `audit_logs`: Audit trail with TTL
  - Indexes on frequently queried columns

#### Deployment
- Kubernetes manifests (Helm charts)
- Minikube support for local development
- Docker images:
  - `ksam-agent:latest`
  - `ksam-core:latest`
  - `ksam-dashboard:latest`

#### Technical Stack
- **Frontend**: React 18, TypeScript, Vite, Tailwind CSS, Cytoscape.js, React Query
- **Backend**: Go 1.21+, Gin, GORM, gRPC
- **Database**: PostgreSQL 15
- **Infrastructure**: Kubernetes, Docker, Helm

---

## [Unreleased]

### 🔜 Planned for v2.0.0 (Q1 2025)

#### RBAC Expansion
- Manage full Kubernetes RBAC pipeline:
  - Roles and ClusterRoles (viewing, editing, creation)
  - RoleBindings and ClusterRoleBindings
  - Pod → ServiceAccount mapping visualization
- RBAC Audit Log lifecycle tracking
- RBAC Graph with relationship visualization
- Permission management interface
- **RBAC Drift Detection**: Detect and alert on unusual permission changes

#### Performance Optimizations
- Code splitting for faster initial load
- Lazy loading for graph components
- Virtual scrolling for large lists
- Worker threads for heavy computations

#### Enhanced Monitoring
- Metrics dashboard (Prometheus integration)
- Real-time alerts for RBAC changes
- Audit log analytics and trends

#### Multi-Cluster Support
- Unified view across multiple clusters
- Cluster comparison tools
- Cross-cluster RBAC analysis

---

## Version History

| Version | Release Date | Key Changes |
|---------|--------------|-------------|
| **1.0.2** | 2024-11-19 | Fixed namespace filter, removed debug logs |
| **1.0.1** | 2024-11-19 | Dark mode, RBAC guards, branding updates, graph fixes |
| **1.0.0** | 2024-11-18 | Initial release with core features |

---

## Breaking Changes

### None so far
All releases are backward compatible.

---

## Migration Guide

### From 1.0.0 to 1.0.1
No migration needed. Simply update the dashboard image:
```bash
kubectl patch deployment ksam-dashboard -n ksam \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"ksam-dashboard:v1.0.1"}]'
```

### From 1.0.1 to 1.0.2
No migration needed. Update the dashboard image:
```bash
kubectl patch deployment ksam-dashboard -n ksam \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"ksam-dashboard:v1.0.2"}]'
```

---

## Support

- **Issues**: Report bugs via GitHub Issues
- **Documentation**: See `docs/` directory
- **Community**: Join our Slack channel (coming soon)

---

**Project:** K8s Workload Management Platform (KSAM)  
**Maintainers:** KSAM Development Team  
**License:** MIT (or your chosen license)

