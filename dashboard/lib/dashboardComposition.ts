import type { PersonaId, PermUser } from './persona';
import { canAny, P } from './permissions';
import type { DashboardDataLoadPolicy } from '../pages/dashboard/types';
import type { DashboardMode, DashboardSectionId } from '../pages/dashboard/types';
import type { OwnershipContext } from './ownershipContext';
import type { TelemetryContext } from './telemetryContext';
import { evaluateVisibility, type VisibilityResult } from './visibilityEngine';
import { evaluateOperationalRelevance, type OperationalRelevanceSignals } from './operationalRelevance';
import type { FeatureId } from './featureRegistry';

export type DashboardWidgetId =
  | 'persona_strip'
  | 'hero_metrics'
  | 'risk_stats'
  | 'entry_points'
  | 'exposure_trend'
  | 'attack_analysis'
  | 'cluster_health'
  | 'activity_feed'
  | 'telemetry_health';

const WIDGET_FEATURES: Record<DashboardWidgetId, FeatureId> = {
  persona_strip: 'dashboard',
  hero_metrics: 'dashboard',
  risk_stats: 'risk_operations',
  entry_points: 'attack_paths',
  exposure_trend: 'risk_operations',
  attack_analysis: 'attack_paths',
  cluster_health: 'clusters',
  activity_feed: 'dashboard',
  telemetry_health: 'monitoring',
};

const PERSONA_WIDGETS: Record<PersonaId, DashboardWidgetId[]> = {
  viewer: ['persona_strip', 'hero_metrics', 'risk_stats', 'entry_points', 'exposure_trend', 'activity_feed'],
  operator: [
    'persona_strip',
    'hero_metrics',
    'risk_stats',
    'entry_points',
    'exposure_trend',
    'attack_analysis',
    'activity_feed',
  ],
  admin: [
    'persona_strip',
    'hero_metrics',
    'risk_stats',
    'entry_points',
    'exposure_trend',
    'attack_analysis',
    'cluster_health',
    'activity_feed',
    'telemetry_health',
  ],
  user_admin: [],
};

const PERSONA_SECTIONS: Record<PersonaId, DashboardSectionId[]> = {
  viewer: ['risk-overview', 'entry-points', 'cluster-health', 'exposure'],
  operator: ['risk-overview', 'entry-points', 'exposure', 'attack-analysis', 'activity'],
  admin: ['risk-overview', 'entry-points', 'cluster-health', 'exposure', 'attack-analysis', 'activity'],
  user_admin: [],
};

const PERSONA_DEFAULT_MODE: Record<PersonaId, DashboardMode> = {
  viewer: 'overview',
  operator: 'full',
  admin: 'full',
  user_admin: 'overview',
};

export interface DashboardComposition {
  defaultMode: DashboardMode;
  sections: DashboardSectionId[];
  widgets: DashboardWidgetId[];
  widgetVisibility: Partial<Record<DashboardWidgetId, VisibilityResult>>;
  hideRuntimeToggles: boolean;
  hideBulkFilters: boolean;
  pageVisible: boolean;
  pageSemantic: VisibilityResult;
}

export function getDashboardComposition(
  personaId: PersonaId,
  user: PermUser,
  ownership: OwnershipContext,
  telemetry: TelemetryContext,
  signals?: OperationalRelevanceSignals,
): DashboardComposition {
  const pageSemantic = evaluateVisibility({
    feature: 'dashboard',
    user,
    personaId,
    ownership,
    telemetry,
  });

  const widgetVisibility: Partial<Record<DashboardWidgetId, VisibilityResult>> = {};
  const widgets: DashboardWidgetId[] = [];

  for (const w of PERSONA_WIDGETS[personaId] ?? []) {
    const feat = WIDGET_FEATURES[w];
    const v = evaluateVisibility({ feature: feat, user, personaId, ownership, telemetry });
    const rel = evaluateOperationalRelevance({
      feature: feat,
      user,
      personaId,
      ownership,
      telemetry,
      signals,
    });
    widgetVisibility[w] = v;
    if (v.visible && rel.relevant) widgets.push(w);
  }

  return {
    defaultMode: PERSONA_DEFAULT_MODE[personaId] ?? 'overview',
    sections: PERSONA_SECTIONS[personaId] ?? [],
    widgets,
    widgetVisibility,
    hideRuntimeToggles: personaId === 'viewer',
    hideBulkFilters: personaId === 'viewer',
    pageVisible: pageSemantic.visible,
    pageSemantic,
  };
}

export function isDashboardSectionVisible(
  personaId: PersonaId,
  sectionId: DashboardSectionId,
  mode: DashboardMode,
  widgets: DashboardWidgetId[],
): boolean {
  const sections = PERSONA_SECTIONS[personaId] ?? [];
  if (!sections.includes(sectionId)) return false;
  if (sectionId === 'attack-analysis' && !widgets.includes('attack_analysis')) return false;
  if (sectionId === 'exposure' && !widgets.includes('exposure_trend')) return false;
  if (sectionId === 'cluster-health' && !widgets.includes('cluster_health')) return false;
  if (sectionId === 'attack-analysis' && mode === 'overview') return false;
  return true;
}

export function isDashboardWidgetVisible(
  composition: DashboardComposition,
  widgetId: DashboardWidgetId,
): boolean {
  return composition.widgets.includes(widgetId);
}

/** Fetch only APIs for widgets the user can see (permission + composition). */
export function getPersonaWidgetCandidates(personaId: PersonaId): DashboardWidgetId[] {
  return PERSONA_WIDGETS[personaId] ?? [];
}

export function buildDashboardLoadPolicy(
  user: PermUser,
  widgets: DashboardWidgetId[],
): DashboardDataLoadPolicy {
  const attackCapable = canAny(user, [P.graphReadSummary, P.graphReadPaths]);
  const monitoringCapable = canAny(user, [
    P.observabilityMetricsRead,
    P.observabilityLogsRead,
    P.observabilityAgentsRead,
  ]);
  const wantsAttack =
    widgets.includes('entry_points') || widgets.includes('attack_analysis');
  const wantsPipeline =
    widgets.includes('telemetry_health') ||
    widgets.includes('entry_points') ||
    widgets.includes('exposure_trend');
  return {
    attackBundle: attackCapable && wantsAttack,
    pipelineHealth: monitoringCapable && wantsPipeline,
    pce: attackCapable && widgets.includes('attack_analysis'),
    entryPods: widgets.includes('entry_points'),
  };
}
