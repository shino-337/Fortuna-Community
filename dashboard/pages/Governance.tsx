import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { api } from '../lib/api';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { usePersona } from '../hooks/usePersona';
import { GovernancePersonaStrip } from '../components/GovernancePersonaStrip';
import { formatDateTime } from '../lib/display';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';
import type { SecurityActivityItem } from '../types';
import { PAGE_TITLES } from '../lib/pageTitles';

type PermRow = {
  permission: string;
  level: string;
  destructive: boolean;
  riskBand: string;
  roles: string[];
  routeCount: number;
  routes: string[];
  graphTierExposure: boolean;
};

type AccessSignal = { code: string; severity: string; userId?: number; username?: string; detail?: Record<string, unknown> };

type CorrSig = { code: string; severity: string; detail?: Record<string, unknown> };

export const Governance: React.FC = () => {
  const permUser = usePermUser();
  const allowed = can(permUser, P.systemAuditRead);
  const { id: personaId } = usePersona();
  const [tab, setTab] = useState<'explorer' | 'access' | 'intel' | 'timeline'>('explorer');
  const [permRows, setPermRows] = useState<PermRow[]>([]);
  const [accessSignals, setAccessSignals] = useState<AccessSignal[]>([]);
  const [corr, setCorr] = useState<CorrSig[]>([]);
  const [events, setEvents] = useState<SecurityActivityItem[]>([]);
  const [domain, setDomain] = useState('');
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');

  const loadExplorer = useCallback(async () => {
    try {
      const d = await api.getGovernancePermissionExplorer();
      setPermRows(d.items || []);
    } catch (err) {
      setErr('Permission explorer failed: ' + String(err));
      setPermRows([]);
    }
  }, []);
  const loadAccess = useCallback(async () => {
    try {
      const d = await api.getGovernanceAccessReview();
      setAccessSignals(d.signals || []);
    } catch (err) {
      setErr('Access review failed: ' + String(err));
      setAccessSignals([]);
    }
  }, []);
  const loadCorr = useCallback(async () => {
    try {
      const d = await api.getGovernanceCorrelationSignals();
      setCorr(d.signals || []);
    } catch (err) {
      setErr('Correlation signals failed: ' + String(err));
      setCorr([]);
    }
  }, []);
  const loadTimeline = useCallback(async () => {
    try {
      const d = await api.getInvestigationEvents({ limit: 80, domain: domain || undefined });
      setEvents(d.items || []);
    } catch (err) {
      setErr('Timeline events failed: ' + String(err));
      setEvents([]);
    }
  }, [domain]);

  useEffect(() => {
    if (!allowed) return;
    setBusy(true);
    setErr('');
    const run = async () => {
      try {
        if (tab === 'explorer') await loadExplorer();
        if (tab === 'access') await loadAccess();
        if (tab === 'intel') await loadCorr();
        if (tab === 'timeline') await loadTimeline();
      } catch (e) {
        setErr(String(e instanceof Error ? e.message : e));
      } finally {
        setBusy(false);
      }
    };
    void run();
  }, [allowed, tab, loadExplorer, loadAccess, loadCorr, loadTimeline]);

  if (!allowed) {
    return (
      <PageLayout title={PAGE_TITLES.governance} description="Requires system.audit.read.">
        <Card className="p-6 text-muted">You do not have permission to view governance analytics.</Card>
      </PageLayout>
    );
  }

  return (
    <PageLayout
      title={PAGE_TITLES.governance}
      description="Authorization assurance, privilege analytics, and investigation views. Data is server-authoritative; API enforces system.audit.read."
    >
      {personaId === 'admin' ? <GovernancePersonaStrip /> : null}
      <div className="flex flex-wrap gap-2 border-b border-border pb-3 mb-4">
        {(['explorer', 'access', 'intel', 'timeline'] as const).map((k) => (
          <Button key={k} size="sm" variant={tab === k ? 'primary' : 'secondary'} type="button" onClick={() => setTab(k)}>
            {k === 'explorer' && 'Permission explorer'}
            {k === 'access' && 'Access review'}
            {k === 'intel' && 'Correlation signals'}
            {k === 'timeline' && 'Investigation timeline'}
          </Button>
        ))}
      </div>
      {err ? <div className="text-red-500 text-caption mb-3">{err}</div> : null}
      {busy ? <div className="text-muted text-caption mb-3">Loading…</div> : null}

      {tab === 'explorer' && (
        <Card className="p-0 overflow-hidden">
          <div className="ui-table-scroll max-h-[70vh]">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH}>Permission</th>
                  <th className={UI_TH}>Risk</th>
                  <th className={UI_TH}>Level</th>
                  <th className={UI_TH}>Roles</th>
                  <th className={UI_TH}>Routes</th>
                </tr>
              </thead>
              <tbody>
                {permRows.map((r) => (
                  <tr key={r.permission} className={UI_TR}>
                    <td className={`${UI_TD} font-mono text-caption`}>{r.permission}</td>
                    <td className={UI_TD}>{r.riskBand}</td>
                    <td className={UI_TD}>{r.destructive ? 'destructive' : r.level}</td>
                    <td className={UI_TD}>{(r.roles || []).join(', ')}</td>
                    <td className={`${UI_TD} text-caption max-w-md`} title={(r.routes || []).join('\n')}>
                      {r.routeCount} · {(r.routes || []).slice(0, 2).join('; ')}
                      {r.routeCount > 2 ? '…' : ''}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {tab === 'access' && (
        <Card className="p-4">
          <ul className="space-y-2">
            {accessSignals.length === 0 ? <li className="text-muted text-caption">No hygiene signals for current heuristics.</li> : null}
            {accessSignals.map((s, i) => (
              <li key={`${s.code}-${i}`} className="text-body border-b border-border/60 pb-2">
                <span className="font-semibold">{s.code}</span>{' '}
                <span className="text-caption text-muted">({s.severity})</span>
                {s.username ? <span className="text-caption"> — {s.username}</span> : null}
              </li>
            ))}
          </ul>
        </Card>
      )}

      {tab === 'intel' && (
        <Card className="p-4">
          <ul className="space-y-2">
            {corr.length === 0 ? <li className="text-muted text-caption">No burst patterns in the last 24h window.</li> : null}
            {corr.map((s, i) => (
              <li key={`${s.code}-${i}`} className="text-caption">
                <span className="font-medium text-text">{s.code}</span> ({s.severity}) — {JSON.stringify(s.detail)}
              </li>
            ))}
          </ul>
        </Card>
      )}

      {tab === 'timeline' && (
        <Card className="p-4 space-y-3">
          <div className="flex flex-wrap gap-2 items-center">
            <label className="text-caption text-muted">Domain</label>
            <select
              className="bg-base border border-border rounded px-2 py-1 text-body"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
            >
              <option value="">all</option>
              <option value="graph">graph</option>
              <option value="export">export</option>
              <option value="rbac">rbac</option>
              <option value="findings">findings</option>
              <option value="sessions">sessions</option>
              <option value="auth">auth</option>
            </select>
            <Button size="sm" variant="secondary" type="button" onClick={() => void loadTimeline()}>
              Refresh
            </Button>
          </div>
          <div className="ui-table-scroll max-h-[65vh]">
            <table className={UI_TABLE}>
              <thead className={UI_THEAD_STICKY}>
                <tr>
                  <th className={UI_TH}>Time</th>
                  <th className={UI_TH}>Action</th>
                  <th className={UI_TH}>Result</th>
                  <th className={UI_TH}>Actor</th>
                </tr>
              </thead>
              <tbody>
                {events.map((e) => (
                  <tr key={`${e.id}-${e.eventId}`} className={UI_TR}>
                    <td className={UI_TD}>{e.createdAt ? formatDateTime(e.createdAt) : '—'}</td>
                    <td className={`${UI_TD} font-mono text-caption`}>{e.action}</td>
                    <td className={UI_TD}>{e.result}</td>
                    <td className={UI_TD}>{e.actorUsername || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </PageLayout>
  );
};
