/**
 * Route manifest — maps paths to features for gating and redirects.
 */
import type { FeatureId } from './featureRegistry';

export interface RouteManifestEntry {
  path: string;
  feature: FeatureId;
  /** More specific paths first when matching. */
  exact?: boolean;
}

export const ROUTE_MANIFEST: RouteManifestEntry[] = [
  { path: '/investigation', feature: 'investigation' },
  { path: '/risks/findings', feature: 'risk_operations' },
  { path: '/risks/pce', feature: 'risk_operations' },
  { path: '/risks/evidence', feature: 'risk_operations' },
  { path: '/risks', feature: 'risk_operations', exact: true },
  { path: '/attack-paths', feature: 'attack_paths' },
  { path: '/network-activity', feature: 'network_activity' },
  { path: '/monitoring', feature: 'monitoring' },
  { path: '/governance', feature: 'governance' },
  { path: '/resources', feature: 'resources' },
  { path: '/clusters', feature: 'clusters' },
  { path: '/capabilities', feature: 'capabilities' },
  { path: '/rules', feature: 'rules_catalog' },
  { path: '/reports', feature: 'reports' },
  { path: '/settings', feature: 'settings' },
  { path: '/certificates', feature: 'certificates' },
  { path: '/', feature: 'dashboard', exact: true },
];

export function featureForPath(pathname: string): FeatureId {
  const path = (pathname.split('?')[0] || '/').replace(/\/$/, '') || '/';
  const sorted = [...ROUTE_MANIFEST].sort((a, b) => b.path.length - a.path.length);
  for (const entry of sorted) {
    if (entry.exact) {
      if (path === entry.path || (entry.path === '/' && path === '')) return entry.feature;
      continue;
    }
    if (path === entry.path || path.startsWith(`${entry.path}/`)) {
      return entry.feature;
    }
  }
  return 'dashboard';
}
