import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Briefcase, ShieldAlert, Activity, FileText, ArrowRight } from 'lucide-react';
import { usePersona } from '../../hooks/usePersona';
import { api } from '../../lib/api';
import { useCan } from '../../hooks/usePermUser';
import { P } from '../../lib/permissions';
import { Button } from '../../components/ui/Button';

/** Role-adaptive dashboard header strip: one composition, persona-driven content. */
export const PersonaDashboardStrip: React.FC = () => {
  const { id, profile } = usePersona();
  const navigate = useNavigate();
  const canInvestigations = useCan(P.investigationsRead);
  const [invStats, setInvStats] = useState({ openCases: 0, overdueRemediation: 0 });

  useEffect(() => {
    if (!canInvestigations || id === 'user_admin') return;
    let cancelled = false;
    api
      .getInvestigationCaseStats()
      .then((s) => {
        if (!cancelled) setInvStats(s);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [canInvestigations, id]);

  if (id === 'viewer') {
    return (
      <div className="rounded-lg border border-brand/25 bg-brand/5 px-4 py-3 flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-caption font-semibold text-text">Exposure narrative</p>
          <p className="text-meta text-muted mt-0.5 max-w-xl">{profile.description}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={() => navigate('/attack-paths')}>
            Attack paths
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/reports')}>
            <FileText className="w-4 h-4 mr-1" />
            Executive brief
          </Button>
        </div>
      </div>
    );
  }

  if (id === 'operator') {
    return (
      <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-4">
          <span className="inline-flex items-center gap-1.5 text-caption text-text font-medium">
            <Briefcase className="w-4 h-4 text-amber-300" />
            {invStats.openCases} open investigation{invStats.openCases !== 1 ? 's' : ''}
          </span>
          {invStats.overdueRemediation > 0 ? (
            <span className="text-caption text-rose-300">
              {invStats.overdueRemediation} overdue remediation
            </span>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={() => navigate('/investigation')}>
            Investigations <ArrowRight className="w-3.5 h-3.5 ml-1" />
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/risks/findings')}>
            <ShieldAlert className="w-4 h-4 mr-1" />
            Triage queue
          </Button>
        </div>
      </div>
    );
  }

  if (id === 'admin') {
    return (
      <div className="rounded-lg border border-violet-500/30 bg-violet-500/10 px-4 py-3 flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-caption font-semibold text-text inline-flex items-center gap-1.5">
            <Activity className="w-4 h-4 text-violet-300" />
            Platform integrity
          </p>
          <p className="text-meta text-muted mt-0.5">
            Prioritize ingestion health, telemetry gaps, and investigation SLA oversight.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" size="sm" onClick={() => navigate('/monitoring')}>
            Monitoring
          </Button>
          <Button variant="secondary" size="sm" onClick={() => navigate('/governance')}>
            Governance
          </Button>
          {canInvestigations ? (
            <Link to="/investigation" className="text-caption text-brand hover:underline self-center">
              Investigations ({invStats.openCases} open)
            </Link>
          ) : null}
        </div>
      </div>
    );
  }

  return null;
};
