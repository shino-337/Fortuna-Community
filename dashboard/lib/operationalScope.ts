/**
 * Server-authoritative operational scope (from login /me).
 * Fallback: parse scopeJson client-side when operationalScope is absent (legacy sessions).
 */
import type { User } from '../types';
import { parseScopeDocument, type ScopeDocument } from './ownershipContext';

export interface OperationalScope {
  clusters: string[];
  namespaces: string[];
  teams: string[];
  environments: string[];
  business_services?: string[];
  crown_jewels?: string[];
  regulatory_domains?: string[];
  restricted: boolean;
}

export function emptyOperationalScope(): OperationalScope {
  return { clusters: [], namespaces: [], teams: [], environments: [], restricted: false };
}

/** Build scope from API field or scopeJson fallback. */
export function resolveOperationalScope(user: User | null | undefined): OperationalScope {
  if (!user) return emptyOperationalScope();
  const fromApi = (user as User & { operationalScope?: OperationalScope }).operationalScope;
  if (fromApi && typeof fromApi === 'object') {
    return {
      clusters: fromApi.clusters ?? [],
      namespaces: fromApi.namespaces ?? [],
      teams: fromApi.teams ?? [],
      environments: fromApi.environments ?? [],
      business_services: fromApi.business_services ?? [],
      crown_jewels: fromApi.crown_jewels ?? [],
      regulatory_domains: fromApi.regulatory_domains ?? [],
      restricted: Boolean(fromApi.restricted),
    };
  }
  return operationalScopeFromDocument(parseScopeDocument(user.scopeJson));
}

export function operationalScopeFromDocument(doc: ScopeDocument): OperationalScope {
  const clusters = [...(doc.clusters ?? []), ...(doc.cluster_ids ?? [])].filter(
    (id, i, arr) => id && arr.indexOf(id) === i,
  );
  const namespaces = doc.namespaces ?? [];
  const teams = doc.teams ?? doc.tenants ?? [];
  const environments = doc.environments ?? [];
  const business_services = doc.business_services ?? [];
  const crown_jewels = doc.crown_jewels ?? [];
  const regulatory_domains = doc.regulatory_domains ?? [];
  const restricted =
    clusters.length > 0 ||
    namespaces.length > 0 ||
    teams.length > 0 ||
    environments.length > 0 ||
    business_services.length > 0 ||
    crown_jewels.length > 0;
  return {
    clusters,
    namespaces,
    teams,
    environments,
    business_services,
    crown_jewels,
    regulatory_domains,
    restricted,
  };
}

export function scopeSummaryLabel(scope: OperationalScope, clusterName?: string): string {
  if (clusterName?.trim()) return clusterName.trim();
  if (scope.clusters.length === 1) return `cluster ${scope.clusters[0]}`;
  if (scope.clusters.length > 1) return `${scope.clusters.length} assigned clusters`;
  if (scope.restricted) return 'your assigned scope';
  return 'all monitored clusters';
}
