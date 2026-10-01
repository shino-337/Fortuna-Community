/**
 * Layer 3 — Ownership scope (operational domain, not security authority).
 */
import type { Cluster } from '../types';
import type { PermUser } from './persona';
import { normalizeFortunaRoleKey } from './fortunaRoles';
import { resolveOperationalScope, type OperationalScope } from './operationalScope';

/** Parsed scope_json (v2 — aligned with core/pkg/authorization/scopedocument.go). */
export interface ScopeDocument {
  clusters?: string[];
  cluster_ids?: string[];
  namespaces?: string[];
  environments?: string[];
  teams?: string[];
  tenants?: string[];
  business_services?: string[];
  crown_jewels?: string[];
  regulatory_domains?: string[];
  labels?: Record<string, string>;
}

export interface OwnershipContext {
  username: string;
  operationalScope: OperationalScope;
  /** User has explicit cluster allow-list. */
  restrictsClusters: boolean;
  allowedClusterIds: string[];
  restrictsNamespaces: boolean;
  allowedNamespaces: string[];
  /** At least one cluster exists in platform inventory. */
  platformHasClusters: boolean;
  /** User can see at least one cluster in their effective scope. */
  hasOperationalScope: boolean;
  /** Selected cluster (header) is outside allowed scope. */
  selectedClusterOutOfScope: boolean;
  selectedClusterId: string | null;
}

export function parseScopeDocument(raw: string | undefined | null): ScopeDocument {
  if (!raw || !String(raw).trim() || String(raw).trim() === '{}') return {};
  const invalid = (): ScopeDocument => ({ clusters: ['__invalid_scope__'] });
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return invalid();
    const o = parsed as Record<string, unknown>;
    const allowed = new Set([
      'clusters',
      'cluster_ids',
      'namespaces',
      'environments',
      'tenants',
      'business_services',
      'crown_jewels',
      'regulatory_domains',
      'labels',
    ]);
    if (Object.keys(o).some((key) => !allowed.has(key))) return invalid();

    const readList = (key: string): string[] | undefined | null => {
      if (!(key in o)) return undefined;
      const value = o[key];
      if (!Array.isArray(value)) return null;
      const out: string[] = [];
      for (const item of value) {
        if (typeof item !== 'string' || !item.trim()) return null;
        out.push(item.trim());
      }
      return out;
    };

    const clusters = readList('clusters');
    const clusterIds = readList('cluster_ids');
    const namespaces = readList('namespaces');
    const environments = readList('environments');
    const tenants = readList('tenants');
    const businessServices = readList('business_services');
    const crownJewels = readList('crown_jewels');
    const regulatoryDomains = readList('regulatory_domains');
    if ([clusters, clusterIds, namespaces, environments, tenants, businessServices, crownJewels, regulatoryDomains].some((v) => v === null)) {
      return invalid();
    }

    let labels: Record<string, string> | undefined;
    if ('labels' in o) {
      const value = o.labels;
      if (!value || typeof value !== 'object' || Array.isArray(value)) return invalid();
      labels = {};
      for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
        if (!key.trim() || typeof item !== 'string') return invalid();
        labels[key] = item;
      }
    }

    return {
      clusters: clusters ?? undefined,
      cluster_ids: clusterIds ?? undefined,
      namespaces: namespaces ?? undefined,
      environments: environments ?? undefined,
      tenants: tenants ?? undefined,
      business_services: businessServices ?? undefined,
      crown_jewels: crownJewels ?? undefined,
      regulatory_domains: regulatoryDomains ?? undefined,
      labels,
    };
  } catch {
    return invalid();
  }
}

function effectiveClusterIds(scope: OperationalScope, role: string, allClusterIds: string[]): string[] {
  if (role === 'admin') return allClusterIds;
  if (scope.clusters.length > 0) {
    if (scope.clusters.length === 1 && scope.clusters[0] === '__invalid_scope__') return [];
    return scope.clusters.map(String);
  }
  if (scope.restricted) return [];
  return allClusterIds;
}

export function buildOwnershipContext(
  user: PermUser,
  clusters: Cluster[],
  selectedClusterId: string | null,
): OwnershipContext {
  const username = user?.username ?? user?.email ?? '';
  const role = normalizeFortunaRoleKey(user?.role);
  const operationalScope = resolveOperationalScope(user as Parameters<typeof resolveOperationalScope>[0]);
  const allClusterIds = clusters.map((c) => String(c.id));

  const allowedClusterIds = effectiveClusterIds(operationalScope, role, allClusterIds);
  const restrictsClusters =
    role !== 'admin' && operationalScope.restricted && operationalScope.clusters.length > 0;
  const allowedNamespaces = operationalScope.namespaces ?? [];
  const restrictsNamespaces = role !== 'admin' && allowedNamespaces.length > 0;

  const platformHasClusters = clusters.length > 0;
  const hasOperationalScope =
    role === 'admin'
      ? true
      : !operationalScope.restricted
        ? true
        : platformHasClusters
          ? allowedClusterIds.some((id) => allClusterIds.includes(id))
          : allowedClusterIds.length > 0;

  const selectedClusterOutOfScope =
    selectedClusterId != null &&
    selectedClusterId !== '' &&
    role !== 'admin' &&
    operationalScope.restricted &&
    allowedClusterIds.length > 0 &&
    !allowedClusterIds.includes(String(selectedClusterId));

  return {
    username,
    operationalScope,
    restrictsClusters,
    allowedClusterIds,
    restrictsNamespaces,
    allowedNamespaces,
    platformHasClusters,
    hasOperationalScope,
    selectedClusterOutOfScope,
    selectedClusterId,
  };
}
