import type { PersonaId } from './persona';

/** @deprecated use loadRiskSavedViews(personaId) */
/** @deprecated use loadRiskSavedViews(personaId) */
export const RISK_SAVED_VIEWS_KEY_PREFIX = 'fortuna-risk-saved-views';

export interface RiskFindingsSavedView {
  id: string;
  name: string;
  createdAt: string;
  personaId?: PersonaId;
  filters: {
    statusFilter: 'all' | 'active' | 'resolved' | 'acknowledged';
    riskLevelFilter: string;
    searchTerm: string;
    clusterId?: string;
    sinceMinutes?: number;
  };
}

/** Get the localStorage key for a persona's saved views. */
export function riskSavedViewsStorageKey(personaId: PersonaId): string {
/** Get the localStorage key for a persona's saved views. */
  return `${RISK_SAVED_VIEWS_KEY_PREFIX}-${personaId}-v1`;
}

/** Load persisted saved views from localStorage. */
export function loadRiskSavedViews(personaId: PersonaId): RiskFindingsSavedView[] {
  if (typeof window === 'undefined') return [];
  try {
    const raw = window.localStorage.getItem(riskSavedViewsStorageKey(personaId));
    if (!raw) return [];
    const parsed = JSON.parse(raw) as RiskFindingsSavedView[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

/** Persist saved views to localStorage. */
export function persistRiskSavedViews(personaId: PersonaId, views: RiskFindingsSavedView[]): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(riskSavedViewsStorageKey(personaId), JSON.stringify(views));
  } catch {
    /* quota */
  }
}

/** Generate a unique ID for a saved view. */
export function newSavedViewId(): string {
  return `view_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`;
}

/** @deprecated use loadRiskSavedViews(personaId) */
/** @deprecated use loadRiskSavedViews(personaId) */
/** @deprecated use loadRiskSavedViews(personaId) */
export const RISK_SAVED_VIEWS_KEY = 'fortuna-risk-saved-views-v1';
