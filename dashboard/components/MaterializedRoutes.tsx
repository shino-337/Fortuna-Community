import React from 'react';
import { matchPath, Navigate, useLocation } from 'react-router-dom';
import { useOperationalMaterialization } from '../hooks/useOperationalMaterialization';
import { isRouteMaterialized } from '../lib/routeMaterialization';

const RiskCenter = React.lazy(() => import('../pages/Insights').then((m) => ({ default: m.RiskCenter })));
const Resources = React.lazy(() => import('../pages/Resources').then((m) => ({ default: m.Resources })));
const NetworkActivity = React.lazy(() => import('../pages/NetworkActivity').then((m) => ({ default: m.NetworkActivity })));
const AttackPaths = React.lazy(() => import('../pages/AttackPaths').then((m) => ({ default: m.AttackPaths })));
const Monitoring = React.lazy(() => import('../pages/Metrics').then((m) => ({ default: m.Monitoring })));
const Rules = React.lazy(() => import('../pages/Rules').then((m) => ({ default: m.Rules })));
const Governance = React.lazy(() => import('../pages/Governance').then((m) => ({ default: m.Governance })));
const Settings = React.lazy(() => import('../pages/Settings').then((m) => ({ default: m.Settings })));
const Certificates = React.lazy(() => import('../pages/Certificates').then((m) => ({ default: m.Certificates })));
const Reports = React.lazy(() => import('../pages/Reports').then((m) => ({ default: m.Reports })));
const Notifications = React.lazy(() => import('../pages/Notifications').then((m) => ({ default: m.Notifications })));
const Capabilities = React.lazy(() => import('../pages/Capabilities').then((m) => ({ default: m.Capabilities })));
const CapabilityDetail = React.lazy(() => import('../pages/CapabilityDetail').then((m) => ({ default: m.CapabilityDetail })));
const Clusters = React.lazy(() => import('../pages/Clusters').then((m) => ({ default: m.Clusters })));
const ClusterDetail = React.lazy(() => import('../pages/ClusterDetail').then((m) => ({ default: m.ClusterDetail })));
const RiskDetail = React.lazy(() => import('../pages/RiskDetail').then((m) => ({ default: m.RiskDetail })));
const PodDetail = React.lazy(() => import('../pages/PodDetail').then((m) => ({ default: m.PodDetail })));
const NodeDetail = React.lazy(() => import('../pages/NodeDetail').then((m) => ({ default: m.NodeDetail })));
const IdentityDetail = React.lazy(() => import('../pages/IdentityDetail').then((m) => ({ default: m.IdentityDetail })));
const RuleDetail = React.lazy(() => import('../pages/RuleDetail').then((m) => ({ default: m.RuleDetail })));
const Investigation = React.lazy(() => import('../pages/Investigation').then((m) => ({ default: m.Investigation })));
const Dashboard = React.lazy(() => import('../pages/Dashboard').then((m) => ({ default: m.Dashboard })));

const ROUTES: Array<{ pattern: string; element: React.ReactNode }> = [
  { pattern: '/dashboard', element: <Dashboard /> },
  { pattern: '/clusters', element: <Clusters /> },
  { pattern: '/clusters/:id', element: <ClusterDetail /> },
  { pattern: '/clusters/:clusterId/nodes/:nodeName', element: <NodeDetail /> },
  { pattern: '/resources', element: <Resources /> },
  { pattern: '/network-activity', element: <NetworkActivity /> },
  { pattern: '/resources/pods/uid/:uid', element: <PodDetail /> },
  { pattern: '/resources/pods/:id', element: <PodDetail /> },
  { pattern: '/risks', element: <RiskCenter /> },
  { pattern: '/risks/findings', element: <RiskCenter /> },
  { pattern: '/risks/pce', element: <RiskCenter /> },
  { pattern: '/risks/evidence', element: <RiskCenter /> },
  { pattern: '/investigation', element: <Investigation /> },
  { pattern: '/risks/:id', element: <RiskDetail /> },
  { pattern: '/capabilities', element: <Capabilities /> },
  { pattern: '/capabilities/:id', element: <CapabilityDetail /> },
  { pattern: '/identities/uid/:uid', element: <IdentityDetail /> },
  { pattern: '/identities/:id', element: <IdentityDetail /> },
  { pattern: '/rules', element: <Rules /> },
  { pattern: '/rules/uid/:uid', element: <RuleDetail /> },
  { pattern: '/rules/:id', element: <RuleDetail /> },
  { pattern: '/attack-paths', element: <AttackPaths /> },
  { pattern: '/monitoring', element: <Monitoring /> },
  { pattern: '/governance', element: <Governance /> },
  { pattern: '/settings', element: <Settings /> },
  { pattern: '/certificates', element: <Certificates /> },
  { pattern: '/reports', element: <Reports /> },
  { pattern: '/notifications', element: <Notifications /> },
];

function buildRedirectTarget(path: string, search: string): string | null {
  if (path === '/network') {
    return `/network-activity${search}`;
  }

  const lowerPath = path.toLowerCase();

  if (['/detailpod', '/detailpods', '/detail-pod', '/detail-pods'].includes(lowerPath)) {
    const params = new URLSearchParams(search);
    const uid = params.get('uid') ?? params.get('podUid') ?? params.get('id');
    if (uid) {
      return `/resources/pods/uid/${encodeURIComponent(uid)}${search}`;
    }
  }

  const podAliases = [
    '/detailPod/uid/:uid',
    '/detailPod/:uid',
    '/detailPods/uid/:uid',
    '/detailPods/:uid',
    '/detail-pod/uid/:uid',
    '/detail-pod/:uid',
    '/detail-pods/uid/:uid',
    '/detail-pods/:uid',
  ];

  for (const pattern of podAliases) {
    const match = matchPath({ path: pattern, end: true }, path);
    const uid = match?.params.uid;
    if (uid) {
      return `/resources/pods/uid/${encodeURIComponent(uid)}${search}`;
    }
  }

  return null;
}

/** Routes that are materialized for the current operational context — redirects to unavailable routes. */
export const MaterializedRoutes: React.FC = () => {
  const allowed = useOperationalMaterialization();
  const r = new Set(allowed.allowedRoutes);
  const location = useLocation();
  const path = location.pathname.replace(/\/$/, '') || '/';
  const defaultPath = allowed.defaultRoute.replace(/\/$/, '') || '/';
  const redirectTarget = buildRedirectTarget(path, location.search);

  if (
    redirectTarget &&
    isRouteMaterialized(redirectTarget, allowed.allowedRoutes)
  ) {
    return <Navigate to={redirectTarget} replace />;
  }

  if (path === '/insights' && r.has('/risks')) {
    return <Navigate to="/risks" replace />;
  }

  if (path === '/metrics' && r.has('/monitoring')) {
    return <Navigate to="/monitoring" replace />;
  }

  if (!isRouteMaterialized(path, allowed.allowedRoutes)) {
    if (path === defaultPath) {
      // Guard against redirect-to-self loops while materialization context is settling.
      return null;
    }
    return <Navigate to={allowed.defaultRoute} replace />;
  }

  const route = ROUTES.find(
    (item) => r.has(item.pattern) && matchPath({ path: item.pattern, end: true }, path),
  );

  if (!route) {
    if (path === defaultPath) {
      return null;
    }
    return <Navigate to={allowed.defaultRoute} replace />;
  }

  return <>{route.element}</>;
};
