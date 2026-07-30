import React from 'react';
import { Link } from 'react-router-dom';
import { Shield, Briefcase, Activity } from 'lucide-react';
import type { PersonaId } from '../lib/persona';
import { getRiskWorkspaceConfig } from '../lib/personaRiskWorkspace';

const ICON: Record<PersonaId, React.ReactNode> = {
  viewer: <Shield className="w-4 h-4 text-brand shrink-0" />,
  operator: <Briefcase className="w-4 h-4 text-amber-300 shrink-0" />,
  admin: <Activity className="w-4 h-4 text-violet-300 shrink-0" />,
  user_admin: null,
};

/** Banner showing the risk workspace title and links — varies by persona. */
export const RiskPersonaWorkspaceBanner: React.FC<{ personaId: PersonaId }> = ({ personaId }) => {
  const cfg = getRiskWorkspaceConfig(personaId);
  if (!cfg.bannerTitle) return null;

  return (
    <div
      className="rounded-lg border border-border/80 bg-surface/50 px-3 py-2.5 flex flex-wrap items-center justify-between gap-2"
      role="note"
    >
      <div className="flex items-start gap-2 min-w-0">
        {ICON[personaId]}
        <div>
          <p className="text-caption font-semibold text-text">{cfg.bannerTitle}</p>
          <p className="text-meta text-muted mt-0.5 max-w-2xl">{cfg.bannerBody}</p>
        </div>
      </div>
      <div className="flex flex-wrap gap-2 text-caption">
        {personaId === 'operator' ? (
          <Link to="/investigation" className="text-brand hover:underline font-medium">
            Investigations
          </Link>
        ) : null}
        {personaId === 'viewer' ? (
          <Link to="/reports" className="text-brand hover:underline font-medium">
            Executive brief
          </Link>
        ) : null}
        {personaId === 'admin' ? (
          <Link to="/monitoring" className="text-brand hover:underline font-medium">
            Monitoring
          </Link>
        ) : null}
      </div>
    </div>
  );
};
