import React, { useCallback, useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { api } from '../lib/api';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';
import { PAGE_TITLES } from '../lib/pageTitles';
import { Tabs } from '../design-system/components/Tabs';
import { SecurityActivityPanel } from '../components/audit/SecurityActivityPanel';
import { PlatformAuditLogPanel } from '../components/audit/PlatformAuditLogPanel';
import { AuditAggregatesPanel } from '../components/audit/AuditAggregatesPanel';

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

/**
 * One place for "who did what in Fortuna", in three views:
 * Activity (security activity, filterable by domain), Platform log (summary plus the full log) and Access.
 */
const AUDIT_TABS = [
  { id: 'activity', label: 'Activity' },
  { id: 'platform', label: 'Platform log' },
  { id: 'access', label: 'Access' },
] as const;
type AuditTab = (typeof AUDIT_TABS)[number]['id'];
/** Tab names used before the audit views were merged into three. */
const LEGACY_TABS: Record<string, AuditTab> = {
  timeline: 'activity',
  aggregates: 'platform',
  permissions: 'access',
  signals: 'access',
  explorer: 'access',
  intel: 'access',
};

function parseTab(raw: string | null): AuditTab {
  if (!raw) return 'activity';
  if (raw in LEGACY_TABS) return LEGACY_TABS[raw];
  return AUDIT_TABS.some((t) => t.id === raw) ? (raw as AuditTab) : 'activity';
}

export const Governance: React.FC = () => {
  const permUser = usePermUser();
  const allowed = can(permUser, P.systemAuditRead);
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = parseTab(searchParams.get('tab'));
  const setTab = useCallback(
    (next: AuditTab) => {
      setSearchParams(
        (prev) => {
          const qs = new URLSearchParams(prev);
          qs.set('tab', next);
          return qs;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );
  const [permRows, setPermRows] = useState<PermRow[]>([]);
  const [accessSignals, setAccessSignals] = useState<AccessSignal[]>([]);
  const [corr, setCorr] = useState<CorrSig[]>([]);
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<string[]>([]);

  const loadAccessTab = useCallback(async () => {
    setBusy(true);
    const [explorer, review, signals] = await Promise.allSettled([
      api.getGovernancePermissionExplorer().then((d) => d.items || []),
      api.getGovernanceAccessReview().then((d) => d.signals || []),
      api.getGovernanceCorrelationSignals().then((d) => d.signals || []),
    ]);
    const failed: string[] = [];
    if (explorer.status === 'fulfilled') setPermRows(explorer.value);
    else failed.push(`Permission explorer: ${String(explorer.reason)}`);
    if (review.status === 'fulfilled') setAccessSignals(review.value);
    else failed.push(`Access review: ${String(review.reason)}`);
    if (signals.status === 'fulfilled') setCorr(signals.value);
    else failed.push(`Correlation signals: ${String(signals.reason)}`);
    setErrors(failed);
    setBusy(false);
  }, []);

  useEffect(() => {
    // Activity and Platform log panels load their own data.
    if (!allowed || tab !== 'access') return;
    void loadAccessTab();
  }, [allowed, tab, loadAccessTab]);

  if (!allowed) {
    return (
      <PageLayout title={PAGE_TITLES.governance} description="Requires system.audit.read.">
        <Card className="p-6 text-muted">You do not have permission to view the audit log.</Card>
      </PageLayout>
    );
  }

  return (
    <PageLayout
      title={PAGE_TITLES.governance}
      description="Who did what in Fortuna, what the platform changed, and who can do what. The API enforces system.audit.read."
    >
      <Tabs
        variant="underline"
        ariaLabel="Audit views"
        className="mb-4 border-b border-border"
        items={AUDIT_TABS.map((t) => ({ id: t.id, label: t.label }))}
        value={tab}
        onChange={(id) => setTab(id as AuditTab)}
      />
      {tab === 'activity' ? <SecurityActivityPanel /> : null}
      {tab === 'platform' ? (
        <div className="space-y-4">
          <AuditAggregatesPanel />
          <PlatformAuditLogPanel />
        </div>
      ) : null}

      {tab === 'access' ? (
        <div className="space-y-4">
          {errors.map((e) => (
            <div key={e} className="text-caption text-red-500" role="alert">
              {e}
            </div>
          ))}
          {busy ? <div className="text-caption text-muted">Loading…</div> : null}

          <Card title="Access review" variant="panel" contentClassName="p-0" actions={
            <Button size="sm" variant="secondary" type="button" onClick={() => void loadAccessTab()} disabled={busy}>
              Refresh
            </Button>
          }>
            <ul className="space-y-2 px-4 py-3">
              {accessSignals.length === 0 && !busy ? <li className="text-caption text-muted">No account hygiene issues found.</li> : null}
              {accessSignals.map((s, i) => (
                <li key={`${s.code}-${i}`} className="border-b border-border/60 pb-2 text-body last:border-b-0">
                  <span className="font-semibold">{s.code}</span> <span className="text-caption text-muted">({s.severity})</span>
                  {s.username ? <span className="text-caption"> · {s.username}</span> : null}
                </li>
              ))}
            </ul>
          </Card>

          <Card title="Correlation signals, last 24 hours" variant="panel" contentClassName="p-0">
            <ul className="space-y-2 px-4 py-3">
              {corr.length === 0 && !busy ? <li className="text-caption text-muted">No burst patterns in the last 24 hours.</li> : null}
              {corr.map((s, i) => (
                <li key={`${s.code}-${i}`} className="text-caption">
                  <span className="font-medium text-text">{s.code}</span> ({s.severity}) · {JSON.stringify(s.detail)}
                </li>
              ))}
            </ul>
          </Card>

          <Card title="Permissions by role" variant="panel" contentClassName="p-0">
            <div className="ui-table-scroll overflow-y-auto max-h-[70vh]">
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
                  {permRows.length === 0 && !busy ? (
                    <tr>
                      <td colSpan={5} className={`${UI_TD} text-caption text-muted`}>No permissions returned.</td>
                    </tr>
                  ) : null}
                  {permRows.map((r) => (
                    <tr key={r.permission} className={UI_TR}>
                      <td className={`${UI_TD} font-mono text-caption`}>{r.permission}</td>
                      <td className={UI_TD}>{r.riskBand}</td>
                      <td className={UI_TD}>{r.destructive ? 'destructive' : r.level}</td>
                      <td className={UI_TD}>{(r.roles || []).join(', ')}</td>
                      <td className={`${UI_TD} max-w-md text-caption`} title={(r.routes || []).join('\n')}>
                        {r.routeCount} · {(r.routes || []).slice(0, 2).join('; ')}
                        {r.routeCount > 2 ? '…' : ''}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        </div>
      ) : null}
    </PageLayout>
  );
};
