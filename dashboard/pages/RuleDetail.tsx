import React, { useCallback, useEffect, useState } from 'react';
import { matchPath, useLocation, useNavigate, useParams } from 'react-router-dom';
import { api } from '../lib/api';
import { SecurityRule, Insight } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, ScrollText, ShieldAlert, Info } from 'lucide-react';
import { getSeverityBadgeClass } from '../lib/severity';
import { PageError, PageLoading } from '../design-system/components/PageStatus';
import { DataFreshness } from '../components/DataFreshness';

export const RuleDetail: React.FC = () => {
  const tooltipLabelClass = 'inline-flex items-center gap-1 underline decoration-dotted underline-offset-2 cursor-help';
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
                <ArrowLeft className="w-4 h-4 mr-2" /> Back to Rules
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
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Rules
          </Button>
        </div>
      }
    >
      <Card className="p-6 mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <span className={`inline-flex items-center rounded-md border px-2.5 py-1 text-caption font-semibold capitalize ${getSeverityBadgeClass(rule.severity ?? 'medium')}`}>
            {rule.severity ?? 'medium'}
          </span>
          <span className="text-muted text-body">{rule.enabled ? 'Enabled' : 'Disabled'}</span>
          <span className="text-muted text-body">Matches: {matchCount}</span>
          <span className="text-muted text-body" title="Rule source: db, files or built-in fallback">
            <span className={tooltipLabelClass}>Source <Info className="w-3 h-3" /></span>: {rule.source ?? 'unknown'}
          </span>
          <span className="text-muted text-body" title="Stable rule UID used by /policy/rules/uid/:uid API calls">
            <span className={tooltipLabelClass}>UID <Info className="w-3 h-3" /></span>: <span className="font-mono text-text">{rule.uid ?? rule.id ?? ruleUid}</span>
          </span>
          <span className="text-muted text-body" title="Primary rule = main rule in a shared signature group. Overlapping rule = same signature group, kept for compatibility/tuning.">
            <span className={tooltipLabelClass}>Rule role <Info className="w-3 h-3" /></span>: {rule.isCanonical === false ? `Overlapping rule of ${rule.canonicalRuleId}` : 'Primary rule'}
          </span>
          <span className="text-muted text-body" title="Impacted findings in rolling windows">
            <span className={tooltipLabelClass}>Impacted findings <Info className="w-3 h-3" /></span>: {rule.impactedFindings24h ?? 0} / 24h · {rule.impactedFindings7d ?? 0} / 7d
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
          <ShieldAlert className="w-5 h-5 text-brand" /> Recent violations
        </h3>
        {recentMatches.length > 0 ? (
          <div className="space-y-2">
            {recentMatches.map((m) => (
              <div
                key={m.id}
                className="p-3 rounded-lg border border-border bg-surface/50 hover:border-brand/30 cursor-pointer"
                onClick={() => navigate(`/risks/${m.id}`)}
              >
                <div className="flex items-center justify-between">
                  <span className="font-medium text-text">{m.title}</span>
                  <span className={`px-2 py-0.5 rounded text-caption font-medium ${getSeverityBadgeClass(m.severity)}`}>
                    {m.severity}
                  </span>
                </div>
                {m.description && (
                  <p className="text-text text-caption mt-2 line-clamp-2">{m.description}</p>
                )}
                {m.impact && (
                  <p className="text-muted text-caption mt-1">Target: {m.impact}</p>
                )}
                {m.timestamp && (
                  <p className="text-muted text-caption mt-1">{new Date(m.timestamp).toLocaleString()}</p>
                )}
                {m.evidence && (
                  <pre className="text-caption text-muted mt-2 bg-base/70 border border-border rounded p-2 overflow-x-auto">
                    {JSON.stringify(m.evidence, null, 2)}
                  </pre>
                )}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-muted text-body">No recent violations.</p>
        )}
      </Card>
    </PageLayout>
  );
};
