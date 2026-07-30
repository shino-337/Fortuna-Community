import React from 'react';
import type { FeatureId } from '../lib/featureRegistry';
import { useFeatureVisibility } from '../hooks/useVisibility';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { PageLoading } from '../design-system/components/PageStatus';
import type { TelemetryContext } from '../lib/telemetryContext';

/**
 * Page-level visibility contract — prevents blank shells on forbidden or out-of-scope pages.
 */
export const PageContract: React.FC<{
  feature: FeatureId;
  children: React.ReactNode;
  loading?: boolean;
  loadingMessage?: string;
  dataEmpty?: boolean;
  ownershipEmpty?: boolean;
  telemetry?: Partial<TelemetryContext>;
}> = ({
  feature,
  children,
  loading,
  loadingMessage = 'Loading…',
  dataEmpty,
  ownershipEmpty,
  telemetry,
}) => {
  const visibility = useFeatureVisibility(feature, { dataEmpty, ownershipEmpty, telemetry });

  if (loading) {
    return <PageLoading message={loadingMessage} className="min-h-[40dvh]" />;
  }

  if (!visibility.visible || visibility.semanticState === 'no_permission' || visibility.semanticState === 'no_scope' || visibility.semanticState === 'no_telemetry') {
    return (
      <SemanticEmptyState
        state={visibility.semanticState}
        reason={visibility.reason}
        className="min-h-[40dvh]"
      />
    );
  }

  return (
    <>
      {(visibility.semanticState === 'policy_hidden' ||
        visibility.semanticState === 'sampled' ||
        visibility.semanticState === 'stale') && (
        <div className="mb-4 rounded-lg border border-border/80 bg-surface/40 px-3 py-2 text-caption text-muted">
          {visibility.reason}
        </div>
      )}
      {children}
    </>
  );
};
