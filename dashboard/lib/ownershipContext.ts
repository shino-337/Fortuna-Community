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
  if (!raw || !String(raw).trim()) return {};
  try {
    const o = JSON.parse(raw) as ScopeDocument;
    if (!o || typeof o !== 'object') return {};
    return o;
  } catch {
    return { clusters: ['__invalid_scope__'] };
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
