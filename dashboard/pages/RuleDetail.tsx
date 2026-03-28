import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { SecurityRule, Insight } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, ScrollText, ShieldAlert, Info } from 'lucide-react';
import { getSeverityBadgeClass } from '../lib/severity';

export const RuleDetail: React.FC = () => {
  const tooltipLabelClass = 'inline-flex items-center gap-1 underline decoration-dotted underline-offset-2 cursor-help';
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [rule, setRule] = useState<SecurityRule & { description?: string } | null>(null);
  const [matchCount, setMatchCount] = useState(0);
  const [recentMatches, setRecentMatches] = useState<Insight[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    const data = await api.getRule(id);
    if (data) {
      setRule(data.rule);
      setMatchCount(data.matchCount);
      setRecentMatches(data.recentMatches ?? []);
    } else {
      setRule(null);
    }
    setLoading(false);
  }, [id]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  if (loading || !id) {
    return (
      <div className="flex flex-col justify-center items-center h-[40vh]">
        <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-slate-500 mt-4">Loading rule...</span>
      </div>
    );
  }

  if (!rule) {
    return (
      <PageLayout title="Rule not found" description="The rule may have been removed.">
        <Button variant="secondary" onClick={() => navigate('/rules')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Rules
        </Button>
      </PageLayout>
    );
  }

  return (
    <PageLayout
      title={rule.name ?? rule.id ?? id}
      description={rule.description ?? `Rule ${rule.id}`}
      actions={
        <Button variant="secondary" onClick={() => navigate('/rules')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Rules
        </Button>
      }
    >
      <Card className="p-6 mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <span className={`px-3 py-1 rounded-full text-sm font-medium ${getSeverityBadgeClass(rule.severity ?? 'medium')}`}>
            {rule.severity ?? 'medium'}
          </span>
          <span className="text-slate-400 text-sm">{rule.enabled ? 'Enabled' : 'Disabled'}</span>
          <span className="text-slate-500 text-sm">Matches: {matchCount}</span>
          <span className="text-slate-500 text-sm" title="Rule source: db, files or built-in fallback">
            <span className={tooltipLabelClass}>Source <Info className="w-3 h-3" /></span>: {rule.source ?? 'unknown'}
          </span>
          <span className="text-slate-500 text-sm" title="Primary rule = main rule in a shared signature group. Overlapping rule = same signature group, kept for compatibility/tuning.">
            <span className={tooltipLabelClass}>Rule role <Info className="w-3 h-3" /></span>: {rule.isCanonical === false ? `Overlapping rule of ${rule.canonicalRuleId}` : 'Primary rule'}
          </span>
          <span className="text-slate-500 text-sm" title="Impacted findings in rolling windows">
            <span className={tooltipLabelClass}>Impacted findings <Info className="w-3 h-3" /></span>: {rule.impactedFindings24h ?? 0} / 24h · {rule.impactedFindings7d ?? 0} / 7d
          </span>
        </div>
        {rule.description && <p className="mt-4 text-slate-300 text-sm">{rule.description}</p>}
        {rule.signature && (
          <p className="mt-2 text-slate-500 text-xs font-mono" title="Unique rule signature used to group similar rules">
            Rule signature: {rule.signature}
          </p>
        )}
        {rule.relatedCapabilities && rule.relatedCapabilities.length > 0 && (
          <p className="mt-2 text-slate-400 text-xs" title="Top capability IDs inferred from matched finding evidence">
            Related capabilities: {rule.relatedCapabilities.slice(0, 5).join(', ')}
          </p>
        )}
      </Card>
      <Card className="p-6">
        <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
          <ShieldAlert className="w-5 h-5 text-pink-500" /> Recent violations
        </h3>
        {recentMatches.length > 0 ? (
          <div className="space-y-2">
            {recentMatches.map((m) => (
              <div
                key={m.id}
                className="p-3 rounded-lg border border-slate-800 bg-slate-900/50 hover:border-pink-500/30 cursor-pointer"
                onClick={() => navigate(`/risks/${m.id}`)}
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium text-white">{m.title}</span>
                  <span className={`px-2 py-0.5 rounded text-xs font-medium ${getSeverityBadgeClass(m.severity)}`}>
                    {m.severity}
                  </span>
                </div>
                {m.description && (
                  <p className="text-slate-300 text-xs mt-2 line-clamp-2">{m.description}</p>
                )}
                {m.impact && (
                  <p className="text-slate-400 text-xs mt-1">Target: {m.impact}</p>
                )}
                {m.timestamp && (
                  <p className="text-slate-500 text-xs mt-1">{new Date(m.timestamp).toLocaleString()}</p>
                )}
                {m.evidence && (
                  <pre className="text-[11px] text-slate-400 mt-2 bg-slate-950/70 border border-slate-800 rounded p-2 overflow-x-auto">
                    {JSON.stringify(m.evidence, null, 2)}
                  </pre>
                )}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-slate-500 text-sm">No recent violations.</p>
        )}
      </Card>
    </PageLayout>
  );
};
