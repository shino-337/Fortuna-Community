import React from 'react';
import type { FeatureId } from '../lib/featureRegistry';
import { useFeatureVisibility } from '../hooks/useVisibility';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import type { TelemetryContext } from '../lib/telemetryContext';

/**
 * Renders children only when visibility engine reports visible.
 * Otherwise shows semantic empty state (never a blank shell).
 */
export const FeatureGate: React.FC<{
  feature: FeatureId;
  children: React.ReactNode;
  dataEmpty?: boolean;
  ownershipEmpty?: boolean;
  telemetry?: Partial<TelemetryContext>;
  /** Still render children when sampled/stale but show banner via GraphVisibilityOverlay */
  allowDegraded?: boolean;
  fallback?: React.ReactNode;
  compact?: boolean;
}> = ({
  feature,
  children,
  dataEmpty,
  ownershipEmpty,
  telemetry,
  allowDegraded = false,
  fallback,
  compact,
}) => {
  const visibility = useFeatureVisibility(feature, { dataEmpty, ownershipEmpty, telemetry });

  if (
    visibility.visible &&
    (visibility.semanticState === 'visible' ||
      visibility.semanticState === 'no_data' ||
      (allowDegraded && (visibility.semanticState === 'sampled' || visibility.semanticState === 'stale' || visibility.semanticState === 'policy_hidden')))
  ) {
    return <>{children}</>;
  }

  if (fallback) return <>{fallback}</>;

  return (
    <SemanticEmptyState
      state={visibility.semanticState}
      reason={visibility.reason}
      compact={compact}
    />
  );
};
