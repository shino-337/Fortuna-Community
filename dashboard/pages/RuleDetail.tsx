import React, { useCallback, useEffect, useState } from 'react';
import { matchPath, useLocation, useNavigate, useParams } from 'react-router-dom';
import { api } from '../lib/api';
import { SecurityRule, Insight } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, ShieldAlert } from 'lucide-react';
import { getSeverityBadgeClass } from '../lib/severity';
import { PageError, PageLoading } from '../design-system/components/PageStatus';
import { DataFreshness } from '../components/DataFreshness';
import { When } from '../components/When';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';

const STATUS_LABEL: Record<string, string> = {
  new: 'Active',
  active: 'Active',
  acknowledged: 'In review',
  resolved: 'Resolved',
  dismissed: 'Dismissed',
};

export const RuleDetail: React.FC = () => {
  const { uid, id } = useParams<{ uid?: string; id?: string }>();
  const location = useLocation();
  const uidFromPath = matchPath({ path: '/rules/uid/:uid', end: true }, location.pathname)?.params.uid;
  const idFromPath = matchPath({ path: '/rules/:id', end: true }, location.pathname)?.params.id;
  const ruleUid = uid ?? uidFromPath ?? id ?? idFromPath;
  const navigate = useNavigate();
  const [rule, setRule] = useState<SecurityRule & { description?: string } | null>(null);
  const [matchCount, setMatchCount] = useState(0);
  const [recentMatches, setRecentMatches] = useState<Insight[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  const fetchData = useCallback(async () => {
    if (!ruleUid) return;
    setLoading(true);
    try {
      const data = await api.getRule(ruleUid);
      if (data) {
        setRule(data.rule);
        setMatchCount(data.matchCount);
        setRecentMatches(data.recentMatches ?? []);
        setError(null);
        setUpdatedAt(new Date());
      } else {
        setRule(null);
        setError(null);
      }
    } catch {
      setError('Rule detail could not be refreshed.');
    }
    setLoading(false);
  }, [ruleUid]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  if (loading || !ruleUid) {
    return (
      <PageLayout title="Rule detail" description="Loading policy rule details.">
        <PageLoading message="Loading rule..." className="min-h-[40dvh]" />
      </PageLayout>
    );
  }

  if (error && !rule) {
    return (
      <PageLayout title="Rule unavailable" description="The rule request failed.">
        <PageError
          title="Could not load rule"
          description="Retry the request or return to the rule catalog."
          action={
            <div className="flex flex-wrap items-center justify-center gap-2">
              <Button variant="secondary" onClick={fetchData} isLoading={loading}>Retry rule</Button>
              <Button variant="secondary" onClick={() => navigate('/rules')}>
                <ArrowLeft className="w-4 h-4 mr-2" /> Rules
              </Button>
            </div>
          }
        />
      </PageLayout>
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
      title={rule.name ?? rule.uid ?? rule.id ?? ruleUid}
      description={rule.description ?? `Rule ${rule.uid ?? rule.id}`}
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <DataFreshness updatedAt={updatedAt} loading={loading} error={error} />
          <Button variant="secondary" onClick={fetchData} isLoading={loading}>Refresh</Button>
          <Button variant="secondary" onClick={() => navigate('/rules')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Rules
          </Button>
        </div>
      }
    >
      <Card className="p-6 mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <span
            className={`inline-flex items-center rounded-md border px-2.5 py-1 text-caption font-semibold capitalize ${getSeverityBadgeClass(rule.severity ?? 'medium')}`}
            title="Severity the rule assigns. Each finding's risk level comes from its resource's score."
          >
            {rule.severity ?? 'medium'}
          </span>
          <span
            className={`px-2 py-0.5 rounded text-caption font-medium border ${
              rule.enabled ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-muted bg-surface-2 border-border'
            }`}
          >
            {rule.enabled ? 'Enabled' : 'Disabled'}
          </span>
          <span className="text-muted text-body">
            Findings <span className="text-text tabular-nums">{rule.impactedFindings7d ?? 0}</span> in 7d ·{' '}
            <span className="text-text tabular-nums">{rule.impactedFindings24h ?? 0}</span> in 24h ·{' '}
            <span className="text-text tabular-nums">{matchCount}</span> total
          </span>
          <span
            className="text-muted text-caption font-mono"
            title="Rule id · where the definition is loaded from (db, files or built-in) · Overlapping = same signature as a primary rule, kept for compatibility or tuning"
          >
            {rule.uid ?? rule.id ?? ruleUid}
            {rule.source ? ` · ${rule.source}` : ''}
            {rule.isCanonical === false ? ` · overlaps ${rule.canonicalRuleId}` : ''}
          </span>
        </div>
        {rule.description && <p className="mt-4 text-text text-body">{rule.description}</p>}
        {rule.signature && (
          <p className="mt-2 text-muted text-caption font-mono" title="Unique rule signature used to group similar rules">
            Rule signature: {rule.signature}
          </p>
        )}
        {rule.relatedCapabilities && rule.relatedCapabilities.length > 0 && (
          <p className="mt-2 text-muted text-caption" title="Top capability IDs inferred from matched finding evidence">
            Related capabilities: {rule.relatedCapabilities.slice(0, 5).join(', ')}
          </p>
        )}
      </Card>
      <Card className="p-6">
        <h3 className="text-section-title text-text mb-4 flex items-center gap-2">
          <ShieldAlert className="w-5 h-5 text-brand" /> Recent findings
        </h3>
        {recentMatches.length > 0 ? (
          <div className="overflow-x-auto rounded-lg border border-border">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH}>Resource</th>
                  <th className={UI_TH}>Risk</th>
                  <th className={UI_TH}>Status</th>
                  <th className={UI_TH}>Detected</th>
                </tr>
              </thead>
              <tbody>
                {recentMatches.map((m) => {
                  const res = m.affectedResources?.[0];
                  return (
                    <tr
                      key={m.id}
                      className={`${UI_TR} cursor-pointer`}
                      onClick={() => navigate(`/risks/${m.id}`)}
                      title="Open finding"
                    >
                      <td className={UI_TD}>
                        <div className="text-text font-medium">{res?.name || m.impact || '—'}</div>
                        <div className="text-meta text-muted">
                          {[res?.kind, res?.namespace].filter(Boolean).join(' · ')}
                        </div>
                      </td>
                      <td className={UI_TD}>
                        {m.finalLevel ? (
                          <span className="inline-flex items-center gap-2 whitespace-nowrap">
                            <span className={`px-1.5 py-0.5 rounded border text-caption font-semibold capitalize ${getSeverityBadgeClass(m.finalLevel)}`}>
                              {m.finalLevel}
                            </span>
                            {m.score != null ? <span className="text-caption text-muted tabular-nums">{m.score}</span> : null}
                          </span>
                        ) : (
                          <span className="text-caption text-muted">No score</span>
                        )}
                      </td>
                      <td className={`${UI_TD} text-caption uppercase text-text`}>{STATUS_LABEL[m.status ?? ''] ?? m.status ?? '—'}</td>
                      <td className={`${UI_TD} text-caption text-muted`}><When iso={m.timestamp} /></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-muted text-body">This rule has not raised a finding yet.</p>
        )}
      </Card>
    </PageLayout>
  );
};
