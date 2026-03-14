import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { SecurityRule } from '../types';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageLoading } from '../components/PageLoading';
import { PageEmpty } from '../components/PageEmpty';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { RefreshCw, Search, FlaskConical } from 'lucide-react';
import { getSeverityBadgeClass } from '../lib/severity';

export const Rules: React.FC = () => {
  const navigate = useNavigate();
  const [rules, setRules] = useState<SecurityRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState<'all' | 'enabled' | 'disabled'>('enabled');
  const [severityFilter, setSeverityFilter] = useState<'all' | 'critical' | 'high' | 'medium' | 'low'>('all');
  const [searchTerm, setSearchTerm] = useState('');
  const [sortBy, setSortBy] = useState<'name_asc' | 'severity_desc' | 'status'>('severity_desc');
  const [reloadingRules, setReloadingRules] = useState(false);
  const [testingRuleId, setTestingRuleId] = useState<string | null>(null);

  const loadRules = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const data = await api.getRules();
      setRules(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load rules');
      setRules([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.SBOM_RISK_LIST));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(loadRules, intervalMs, { refreshTrigger });
  useEffect(() => { loadRules(); }, [loadRules]);

  const filteredRules = useMemo(() => {
    let out = [...rules];
    if (statusFilter !== 'all') out = out.filter((r) => (statusFilter === 'enabled' ? r.enabled : !r.enabled));
    if (severityFilter !== 'all') out = out.filter((r) => (r.severity || '').toLowerCase() === severityFilter);
    if (searchTerm.trim()) {
      const q = searchTerm.trim().toLowerCase();
      out = out.filter((r) => [r.id, r.name, r.category, r.type, r.description].some((v) => (v || '').toLowerCase().includes(q)));
    }
    const sevRank: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };
    out.sort((a, b) => {
      switch (sortBy) {
        case 'name_asc':
          return (a.name || '').localeCompare(b.name || '');
        case 'status':
          return Number(b.enabled) - Number(a.enabled);
        case 'severity_desc':
        default:
          return (sevRank[(b.severity || '').toLowerCase()] ?? 0) - (sevRank[(a.severity || '').toLowerCase()] ?? 0);
      }
    });
    return out;
  }, [rules, statusFilter, severityFilter, searchTerm, sortBy]);

  const handleReloadRules = async () => {
    setReloadingRules(true);
    try {
      await api.reloadRules();
      await loadRules();
    } finally {
      setReloadingRules(false);
    }
  };

  const handleQuickTest = async (ruleId: string) => {
    setTestingRuleId(ruleId);
    try {
      await api.testRule(ruleId, {
        apiVersion: 'v1',
        kind: 'Pod',
        metadata: { name: 'quick-test-pod', namespace: 'default' },
      });
    } finally {
      setTestingRuleId(null);
    }
  };

  if (loading) return <PageLoading message="Loading policy rules..." className="min-h-[40vh]" />;

  return (
    <PageLayout
      title="Policy Rules"
      description="Production rule catalog for detections and policy enforcement."
      actions={
        <div className="flex items-center gap-2">
          <Button variant="secondary" isLoading={loading} onClick={loadRules}>
            <RefreshCw className="w-4 h-4 mr-2" /> Refresh
          </Button>
          <Button variant="secondary" isLoading={reloadingRules} onClick={handleReloadRules}>
            Reload Rules Engine
          </Button>
        </div>
      }
      toolbar={
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative min-w-[220px]">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 w-4 h-4" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder="Search id, name, category..."
              className="w-full bg-slate-950 border border-slate-700 rounded-lg pl-9 pr-4 py-2 text-sm text-white focus:ring-2 focus:ring-pink-500 outline-none"
            />
          </div>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as typeof statusFilter)}
            className="bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-300"
          >
            <option value="all">All status</option>
            <option value="enabled">Enabled</option>
            <option value="disabled">Disabled</option>
          </select>
          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value as typeof severityFilter)}
            className="bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-300"
          >
            <option value="all">All severity</option>
            <option value="critical">Critical</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
          </select>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as typeof sortBy)}
            className="bg-slate-950 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-300"
          >
            <option value="severity_desc">Sort: Severity high to low</option>
            <option value="name_asc">Sort: Name A-Z</option>
            <option value="status">Sort: Enabled first</option>
          </select>
        </div>
      }
    >
      {error && (
        <div className="mb-4 rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-amber-200 text-sm">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <Card variant="panel"><div className="text-muted text-[10px] uppercase tracking-wide">Total</div><div className="text-xl font-bold text-text mt-1">{rules.length}</div></Card>
        <Card variant="panel"><div className="text-muted text-[10px] uppercase tracking-wide">Enabled</div><div className="text-xl font-bold text-emerald-400 mt-1">{rules.filter((r) => r.enabled).length}</div></Card>
        <Card variant="panel"><div className="text-muted text-[10px] uppercase tracking-wide">Disabled</div><div className="text-xl font-bold text-muted mt-1">{rules.filter((r) => !r.enabled).length}</div></Card>
      </div>

      <Card className="p-0 overflow-hidden">
        <div className="overflow-x-auto max-h-[60vh] overflow-y-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border">
              <tr>
                <th className="px-6 py-4 font-medium">Rule</th>
                <th className="px-6 py-4 font-medium">Category / Type</th>
                <th className="px-6 py-4 font-medium">Severity</th>
                <th className="px-6 py-4 font-medium">Status</th>
                <th className="px-6 py-4 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800">
              {filteredRules.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-8">
                    <PageEmpty title="No rules match current filters" description="Adjust search or filter conditions." className="py-6" />
                  </td>
                </tr>
              ) : (
                filteredRules.map((rule) => (
                  <tr key={rule.id} className="hover:bg-muted/30 transition-colors">
                    <td className="px-6 py-4">
                      <div className="font-medium text-white">{rule.name}</div>
                      <div className="text-xs text-slate-500 font-mono">{rule.id}</div>
                    </td>
                    <td className="px-6 py-4 text-slate-400">
                      {(rule.category || 'uncategorized')}{rule.type ? ` / ${rule.type}` : ''}
                    </td>
                    <td className="px-6 py-4">
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${getSeverityBadgeClass(rule.severity)}`}>{rule.severity}</span>
                    </td>
                    <td className="px-6 py-4">
                      <span className={`px-2 py-0.5 rounded text-xs font-medium border ${rule.enabled ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20' : 'text-slate-400 bg-slate-800 border-slate-700'}`}>
                        {rule.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <Button size="sm" variant="secondary" onClick={() => handleQuickTest(rule.id)} disabled={testingRuleId === rule.id}>
                          <FlaskConical className="w-4 h-4 mr-1" /> Test
                        </Button>
                        <Button size="sm" onClick={() => navigate(`/rules/${rule.id}`)}>Open</Button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </Card>
    </PageLayout>
  );
};
