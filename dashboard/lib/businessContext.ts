/**
 * Business-aware operational context (crown jewels, regulated systems, services).
 */
import type { OperationalScope } from './operationalScope';

export interface BusinessContext {
  businessServices: string[];
  crownJewels: string[];
  regulatoryDomains: string[];
  /** service id/name → criticality 1–5 */
  serviceCriticality: Record<string, number>;
}

/** Build a BusinessContext from an operational scope. */
export function businessContextFromScope(scope: OperationalScope): BusinessContext {
  const ext = scope as OperationalScope & {
    business_services?: string[];
    crown_jewels?: string[];
    regulatory_domains?: string[];
  };
  const businessServices = ext.business_services ?? [];
  const crownJewels = ext.crown_jewels ?? [];
  const regulatoryDomains = ext.regulatory_domains ?? [];
  const serviceCriticality: Record<string, number> = {};
  for (const s of businessServices) {
    serviceCriticality[s] = crownJewels.includes(s) ? 5 : regulatoryDomains.length > 0 ? 4 : 3;
  }
  for (const cj of crownJewels) {
    serviceCriticality[cj] = 5;
  }
  return { businessServices, crownJewels, regulatoryDomains, serviceCriticality };
}

/** Check if an asset ID matches any crown jewel identifier. */
export function isCrownJewelAsset(assetId: string, ctx: BusinessContext): boolean {
  return ctx.crownJewels.some((c) => assetId.includes(c) || c.includes(assetId));
}

/** Determine if escalation is required for a given environment based on regulatory domains. */
export function regulatedEscalationRequired(ctx: BusinessContext, environment?: string): boolean {
  if (ctx.regulatoryDomains.length === 0) return false;
  return Boolean(environment && ctx.regulatoryDomains.some((d) => environment.toLowerCase().includes(d.toLowerCase())));
}

/** Get a human-readable label for business impact based on criticality. */
export function businessImpactLabel(serviceName: string, ctx: BusinessContext): string {
  const crit = ctx.serviceCriticality[serviceName];
  if (crit == null) return 'Standard workload';
  if (crit >= 5) return 'Crown-jewel service';
  if (crit >= 4) return 'Regulated / high-criticality';
  return 'Business service';
}
