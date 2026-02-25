import React, { useCallback, useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Insight, RuntimeSignal } from '../types';
import { PageLayout } from '../components/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, ShieldAlert, Calendar, FileText, Box, AlertTriangle } from 'lucide-react';
import { getSeverityBadgeClass } from '../lib/severity';
import { useTimeWindowStore } from '../store/timeWindowStore';

export const RiskDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [insight, setInsight] = useState<Insight | null>(null);
  const [loading, setLoading] = useState(true);
  const [resolving, setResolving] = useState(false);
  const [podRuntimeSignals, setPodRuntimeSignals] = useState<RuntimeSignal[]>([]);
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);

  const fetchInsight = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    const data = await api.getInsight(id);
    setInsight(data);
    setLoading(false);
  }, [id]);

  useEffect(() => {
    fetchInsight();
  }, [fetchInsight]);

  // When insight has Pod assets, fetch runtime/escape signals for those pods so risk view shows escape info
  useEffect(() => {
    if (!insight?.affectedResources?.length) {
      setPodRuntimeSignals([]);
      return;
    }
    const podUids = insight.affectedResources
      .filter((r) => r.kind === 'Pod' && r.id)
      .map((r) => r.id as string);
    if (podUids.length === 0) {
      setPodRuntimeSignals([]);
      return;
    }
    let cancelled = false;
    const load = async () => {
      const all: RuntimeSignal[] = [];
      for (const uid of podUids.slice(0, 3)) {
        try {
          const sinceMinutes = timeWindowMinutes > 0 ? timeWindowMinutes : undefined;
      const signals = await api.getRuntimeSignalsByPod(uid, { limit: 10, sinceMinutes });
          if (!cancelled) all.push(...signals);
        } catch {
          // ignore per-pod errors
        }
      }
      if (!cancelled) setPodRuntimeSignals(all);
    };
    load();
    return () => { cancelled = true; };
  }, [insight?.id, insight?.affectedResources, timeWindowMinutes]);

  const handleResolve = async () => {
    if (!id || insight?.status === 'resolved') return;
    setResolving(true);
    try {
      await api.resolveInsight(id);
      await fetchInsight();
    } finally {
      setResolving(false);
    }
  };

  if (loading || !id) {
    return (
      <div className="flex flex-col justify-center items-center h-[40vh]">
        <div className="w-12 h-12 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-slate-500 mt-4">Loading risk...</span>
      </div>
    );
  }

  if (!insight) {
    return (
      <PageLayout title="Risk not found" description="The risk may have been resolved or removed.">
        <Button variant="secondary" onClick={() => navigate('/risks')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to Risk Center
        </Button>
      </PageLayout>
    );
  }

  const severityClass = getSeverityBadgeClass(insight.severity);
  const statusLabelMap: Record<string, string> = {
    new: 'Active',
    acknowledged: 'In review',
    resolved: 'Resolved',
  };

  return (
    <PageLayout
      title={insight.title}
      description={insight.id !== insight.title ? `Finding ID: ${insight.id}` : undefined}
      actions={
        <div className="flex items-center gap-2">
          {insight.status !== 'resolved' && (
            <Button variant="secondary" onClick={handleResolve} disabled={resolving}>
              {resolving ? 'Resolving...' : 'Mark as resolved'}
            </Button>
          )}
          <Button variant="secondary" onClick={() => navigate('/risks')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Risk Center
          </Button>
        </div>
      }
    >
      {/* Summary banner */}
      <Card className="p-6 mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <span className={`px-3 py-1 rounded-full text-sm font-medium ${severityClass}`}>
            {insight.severity}
          </span>
          <span className="text-slate-400 text-sm uppercase">{statusLabelMap[insight.status ?? ''] ?? (insight.status ?? 'Active')}</span>
          {insight.score != null && (
            <span className="text-slate-400 text-sm">Risk score: {insight.score}/100</span>
          )}
        </div>
        {insight.description && (
          <p className="mt-4 text-slate-300 text-sm">{insight.description}</p>
        )}
        {insight.impact && (
          <div className="mt-4 pt-4 border-t border-slate-800">
            <h4 className="text-slate-400 text-xs uppercase mb-1">Recommended action</h4>
            <p className="text-slate-300 text-sm">{insight.impact}</p>
          </div>
        )}
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Affected Assets */}
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Box className="w-5 h-5 text-pink-500" /> Impacted Resources
          </h3>
          {insight.affectedResources?.length ? (
            <ul className="space-y-2">
              {insight.affectedResources.map((r, i) => (
                <li key={r.id || i} className="flex items-center justify-between p-3 bg-slate-900/50 rounded-lg border border-slate-800">
                  <div>
                    <span className="text-white font-medium">{r.name ?? r.id}</span>
                    {r.namespace && <span className="text-slate-500 ml-2">ns/{r.namespace}</span>}
                    {r.kind && <span className="text-slate-500 ml-2">({r.kind})</span>}
                  </div>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      if (r.kind === 'Pod' && r.id) navigate(`/resources/pods/uid/${encodeURIComponent(r.id)}`);
                      if (r.kind === 'ServiceAccount' && r.id) navigate(`/identities/uid/${encodeURIComponent(r.id)}`);
                    }}
                    title={r.kind === 'Pod' ? 'View pod in Resources' : r.kind === 'ServiceAccount' ? 'View identity' : 'View resource'}
                  >
                    {r.kind === 'Pod' ? 'View pod' : r.kind === 'ServiceAccount' ? 'View identity' : 'View'}
                  </Button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-slate-500 text-sm">No impacted resources linked.</p>
          )}
        </Card>

        {/* Timeline */}
        <Card className="p-6">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
            <Calendar className="w-5 h-5 text-pink-500" /> Investigation Timeline
          </h3>
          <dl className="space-y-3 text-sm">
            {insight.timestamp && (
              <div>
                <dt className="text-slate-500">Detected</dt>
                <dd className="text-slate-300">{new Date(insight.timestamp).toLocaleString()}</dd>
              </div>
            )}
            {insight.updatedAt && (
              <div>
                <dt className="text-slate-500">Updated</dt>
                <dd className="text-slate-300">{new Date(insight.updatedAt).toLocaleString()}</dd>
              </div>
            )}
            {insight.resolvedAt && (
              <div>
                <dt className="text-slate-500">Resolved</dt>
                <dd className="text-emerald-400">{new Date(insight.resolvedAt).toLocaleString()}</dd>
              </div>
            )}
            {!insight.timestamp && !insight.updatedAt && !insight.resolvedAt && (
              <p className="text-slate-500">No timeline data available.</p>
            )}
          </dl>
        </Card>
      </div>

      {/* Runtime / Escape signals – for affected Pods so risk view shows escape info when runtime has it */}
      {(insight.affectedResources?.some((r) => r.kind === 'Pod') || podRuntimeSignals.length > 0) && (
        <Card className="p-6 mt-6">
          <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2">
            <AlertTriangle className="w-5 h-5 text-amber-500" /> Runtime Evidence
          </h3>
          <p className="text-slate-500 text-sm mb-4">Runtime events linked to impacted pods (e.g. PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT).</p>
          {podRuntimeSignals.length === 0 ? (
            <p className="text-slate-500 text-sm">No runtime evidence for impacted pods in selected time window.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-slate-500 border-b border-slate-800">
                    <th className="text-left py-2">Event</th>
                    <th className="text-left py-2">Category</th>
                    <th className="text-left py-2">Pod UID</th>
                    <th className="text-left py-2">Date</th>
                  </tr>
                </thead>
                <tbody>
                  {podRuntimeSignals.slice(0, 10).map((s) => (
                    <tr key={s.id} className="border-b border-slate-800/50">
                      <td className="py-1.5 font-medium text-amber-400">{s.signalType}</td>
                      <td className="py-1.5 text-slate-400">{s.category}</td>
                      <td className="py-1.5 text-slate-500 font-mono truncate max-w-[120px]" title={s.podUid}>
                        {s.podUid ? `${s.podUid.slice(0, 8)}…` : '—'}
                      </td>
                      <td className="py-1.5 text-slate-500">{s.createdAt ? new Date(s.createdAt).toLocaleString() : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}

      {/* Evidence / Violated Rules – from backend insights.evidence and insights.violated_rules */}
      {(insight.evidence != null || insight.violatedRules != null) && (
        <Card className="p-6 mt-6">
          <h3 className="text-lg font-semibold text-white mb-2 flex items-center gap-2">
            <FileText className="w-5 h-5 text-pink-500" /> Technical Evidence & Violated Rules
          </h3>
          {insight.evidence != null && (
            <div className="mb-4">
              <h4 className="text-slate-400 text-sm uppercase mb-2">Evidence</h4>
              <pre className="p-3 bg-slate-900/50 rounded border border-slate-800 text-slate-300 text-xs overflow-x-auto">
                {typeof insight.evidence === 'string'
                  ? insight.evidence
                  : JSON.stringify(insight.evidence, null, 2)}
              </pre>
            </div>
          )}
          {insight.violatedRules != null && (
            <div>
              <h4 className="text-slate-400 text-sm uppercase mb-2">Violated Rules</h4>
              <pre className="p-3 bg-slate-900/50 rounded border border-slate-800 text-slate-300 text-xs overflow-x-auto">
                {typeof insight.violatedRules === 'string'
                  ? insight.violatedRules
                  : JSON.stringify(insight.violatedRules, null, 2)}
              </pre>
            </div>
          )}
        </Card>
      )}
    </PageLayout>
  );
};
