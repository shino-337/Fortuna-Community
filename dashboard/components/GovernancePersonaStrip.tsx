import React from 'react';
import { Link } from 'react-router-dom';
import { ScrollText, Shield } from 'lucide-react';

/** Admin governance workspace emphasis (Phase 2). */
export const GovernancePersonaStrip: React.FC = () => (
  <div className="mb-4 rounded-lg border border-violet-500/25 bg-violet-500/10 px-4 py-3 flex flex-wrap gap-3 items-center justify-between">
    <div className="flex items-start gap-2">
      <ScrollText className="w-5 h-5 text-violet-300 shrink-0 mt-0.5" />
      <div>
        <p className="text-caption font-semibold text-text">Governance workspace</p>
        <p className="text-meta text-muted mt-0.5">
          RBAC explorer, access review signals, and investigation event timeline — prioritize platform trust.
        </p>
      </div>
    </div>
    <div className="flex flex-wrap gap-3 text-caption">
      <Link to="/monitoring" className="text-brand hover:underline">
        Ingestion health
      </Link>
      <Link to="/investigation" className="text-brand hover:underline inline-flex items-center gap-1">
        <Shield className="w-3.5 h-3.5" />
        Investigations
      </Link>
    </div>
  </div>
);
