import React from 'react';
import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useOperationalContext } from '../hooks/useOperationalContext';
import { featureForPath } from '../lib/routeManifest';
import { evaluateVisibility } from '../lib/visibilityEngine';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import { getFeature } from '../lib/featureRegistry';

/**
 * Route-level Layer 2–4 gate: permissions, persona, ownership, telemetry.
 */
export const PersonaAccessGate: React.FC = () => {
  const location = useLocation();
  const { user, personaId, profile, ownership, telemetry } = useOperationalContext();
  const feature = featureForPath(location.pathname);
  const visibility = evaluateVisibility({
    feature,
    user,
    personaId,
    ownership,
    telemetry,
  });

  const homeVisibility = evaluateVisibility({
    feature: featureForPath(profile.homeRoute),
    user,
    personaId,
    ownership,
    telemetry,
  });

  const redirectTarget =
    homeVisibility.visible && homeVisibility.semanticState !== 'no_permission'
      ? profile.homeRoute
      : '/settings';

  if (!visibility.visible || visibility.semanticState === 'no_permission') {
    if (location.pathname === redirectTarget) {
      return (
        <SemanticEmptyState
          state={visibility.semanticState}
          reason={visibility.reason}
          title={`${getFeature(feature).label} unavailable`}
          className="min-h-[50dvh]"
        />
      );
    }
    return (
      <Navigate
        to={redirectTarget}
        replace
        state={{ from: location.pathname, reason: visibility.semanticState, feature }}
      />
    );
  }

  if (visibility.semanticState === 'no_scope' || visibility.semanticState === 'no_telemetry') {
    return (
      <SemanticEmptyState
        state={visibility.semanticState}
        reason={visibility.reason}
        title={`${getFeature(feature).label}`}
        className="min-h-[50dvh]"
      />
    );
  }

  return <Outlet />;
};
