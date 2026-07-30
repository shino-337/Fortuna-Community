import type { AttackChain, AttackPath, PodWithRisk } from '../../types';

export type RiskBand = 'critical' | 'high' | 'medium' | 'low';
export type DashboardMode = 'overview' | 'full';
export type RuntimeStatus = 'confirmed' | 'inferred' | 'not_observed';

export interface EntryScenario {
  key: string;
  pod: PodWithRisk;
  chain?: AttackChain;
  paths: AttackPath[];
  pathLabel: string;
  riskLevel: RiskBand;
  score?: number;
  runtimeConfirmed: boolean;
}

export interface RuntimeConfirmationItem {
  id: string;
  label: string;
  status: RuntimeStatus;
  confidence?: number;
  source?: string;
}

export type DashboardSectionId =
  | 'risk-overview'
  | 'entry-points'
  | 'cluster-health'
  | 'exposure'
  | 'attack-analysis'
  | 'activity';

export type DashboardSectionLoadState = 'idle' | 'loading' | 'ready' | 'error' | 'skipped';

/** Which optional dashboard fetches to run (permission + widget visibility). */
export interface DashboardDataLoadPolicy {
  attackBundle: boolean;
  pipelineHealth: boolean;
  pce: boolean;
  entryPods: boolean;
}
