import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import type { Report } from '../types';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { Card } from '../design-system/components/Card';
import {
  FileText,
  Download,
  ArrowLeft,
  Activity,
  Route,
  ShieldAlert,
  ClipboardList,
  Layers,
  Briefcase,
  AlertCircle,
  BarChart3,
  Database,
  RefreshCw,
  ShieldCheck,
  RotateCcw,
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty, PageError } from '../design-system/components/PageStatus';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';
import { PAGE_TITLES } from '../lib/pageTitles';

type ExecutivePosture = {
  stats: {
    clusters?: number;
    pods?: number;
    insights?: number;
    critical?: number;
    totalClusters?: number;
    runningPods?: number;
  } | null;
  summary: { total?: number; critical?: number; high?: number; medium?: number; low?: number } | null;
  pipeline: {
    layer2?: { exploitedCapCount?: number };
    layer3?: { totalPaths?: number; criticalPaths?: number };
  } | null;
};

type ReportRangeDays = 1 | 3 | 7 | 30;

const REPORT_RANGE_OPTIONS: Array<{ days: ReportRangeDays; label: string }> = [
  { days: 1, label: '1 day' },
  { days: 3, label: '3 days' },
  { days: 7, label: '7 days' },
  { days: 30, label: '30 days' },
];

const DEFAULT_REPORT_RANGE_DAYS: ReportRangeDays = 1;

function rangeToHours(days: ReportRangeDays): number {
  return days * 24;
}

function rangeToSinceMinutes(days: ReportRangeDays): number {
  return days * 24 * 60;
}

export const Reports: React.FC = () => {
  const navigate = useNavigate();
  const permUser = usePermUser();
  const canPlatformAudit = can(permUser, P.systemAuditRead);
  const canFindingsRead = can(permUser, P.findingsRead);
  const canExportFindings = can(permUser, P.exportFindings);
  const canViewReports = canFindingsRead || canPlatformAudit;
  const [rangeDays, setRangeDays] = useState<ReportRangeDays>(DEFAULT_REPORT_RANGE_DAYS);
  const [reports, setReports] = useState<Report[]>([]);
  const [loading, setLoading] = useState(true);
  const [postureLoading, setPostureLoading] = useState(true);
  const [auditError, setAuditError] = useState<string | null>(null);
  const [postureError, setPostureError] = useState<string | null>(null);
  const [riskExporting, setRiskExporting] = useState<'csv' | 'pdf' | null>(null);
  const [posture, setPosture] = useState<ExecutivePosture>({ stats: null, summary: null, pipeline: null });
  const canInvestigationsRead = can(permUser, P.investigationsRead);
  const [investigationStats, setInvestigationStats] = useState({
    openCases: 0,
    overdueRemediation: 0,
  });

  const loadAuditReports = useCallback(() => {
    if (!canPlatformAudit) {
      setReports([]);
      setLoading(false);
      setAuditError(null);
      return;
    }
    setLoading(true);
    setAuditError(null);
    api.getReportsStrict({ hours: rangeToHours(rangeDays) })
      .then((data) => {
        setReports(data);
        setLoading(false);
      })
      .catch((err) => {
        setAuditError(err instanceof Error ? err.message : 'Could not load audit aggregates.');
        setReports([]);
        setLoading(false);
      });
  }, [canPlatformAudit, rangeDays]);

  const loadPosture = useCallback(() => {
    if (!canFindingsRead && !canPlatformAudit) {
      setPostureLoading(false);
      return;
    }
    setPostureLoading(true);
    setPostureError(null);
    let cancelled = false;
    const sinceMinutes = rangeToSinceMinutes(rangeDays);
    Promise.allSettled([
      api.getStats(),
      api.getInsightsSummary(null, sinceMinutes),
      api.getPipelineHealth(),
    ]).then(([statsResult, summaryResult, pipelineResult]) => {
      if (cancelled) return;
      setPosture({
        stats: statsResult.status === 'fulfilled' ? statsResult.value : null,
        summary: summaryResult.status === 'fulfilled' ? summaryResult.value : null,
        pipeline: pipelineResult.status === 'fulfilled' ? pipelineResult.value : null,
      });
      // Only show error if key APIs failed (stats + summary)
      if (
        statsResult.status === 'rejected' &&
        summaryResult.status === 'rejected'
      ) {
        setPostureError('Could not load posture APIs.');
      }
    }).finally(() => {
      if (!cancelled) setPostureLoading(false);
    });
    return () => { cancelled = true; };
  }, [canFindingsRead, canPlatformAudit, rangeDays]);

  useEffect(() => {
    loadAuditReports();
  }, [loadAuditReports]);

  useEffect(() => {
    if (!canInvestigationsRead) return;
    let cancelled = false;
    api.getInvestigationCaseStats()
      .then((stats) => {
        if (!cancelled) setInvestigationStats(stats);
      })
      .catch(() => {
        if (!cancelled) setInvestigationStats({ openCases: 0, overdueRemediation: 0 });
      });
    return () => {
      cancelled = true;
    };
  }, [canInvestigationsRead]);

  useEffect(() => {
    return loadPosture();
  }, [loadPosture]);

  const auditTotal = useMemo(
    () => reports.reduce((sum, report) => sum + Number(report.count ?? 0), 0),
    [reports],
  );

  const topAuditAggregate = useMemo(
    () => [...reports].sort((a, b) => Number(b.count ?? 0) - Number(a.count ?? 0))[0],
    [reports],
  );

  const activeFindings = posture.summary?.total;
  const criticalFindings = posture.summary?.critical;
  const highFindings = posture.summary?.high;
  const mediumFindings = posture.summary?.medium;
  const lowFindings = posture.summary?.low;
  const criticalPaths = posture.pipeline?.layer3?.criticalPaths;
  const totalPaths = posture.pipeline?.layer3?.totalPaths;
  const exploitedSignals = posture.pipeline?.layer2?.exploitedCapCount;
  const fleetClusterCount = posture.stats?.clusters ?? posture.stats?.totalClusters;
  const fleetPodCount = posture.stats?.pods ?? posture.stats?.runningPods;
  const unavailablePostureSources = [
    posture.stats ? null : 'stats',
    posture.summary ? null : 'risk summary',
    posture.pipeline ? null : 'pipeline',
  ].filter(Boolean) as string[];
  const auditByResource = useMemo(() => {
    const map = new Map<string, number>();
    for (const report of reports) {
      const key = (report.resource || 'unknown').trim() || 'unknown';
      map.set(key, (map.get(key) ?? 0) + Number(report.count ?? 0));
    }
    return [...map.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6);
  }, [reports]);
  const auditByAction = useMemo(() => {
    const map = new Map<string, number>();
    for (const report of reports) {
      const key = (report.action || 'unknown').trim() || 'unknown';
      map.set(key, (map.get(key) ?? 0) + Number(report.count ?? 0));
    }
    return [...map.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6);
  }, [reports]);
  const maxAuditResourceCount = Math.max(1, ...auditByResource.map(([, count]) => count));
  const maxAuditActionCount = Math.max(1, ...auditByAction.map(([, count]) => count));
  const activeRangeLabel = `${rangeDays} day${rangeDays === 1 ? '' : 's'}`;

  const refreshAll = useCallback(() => {
    loadAuditReports();
    loadPosture();
  }, [loadAuditReports, loadPosture]);

  const resetRange = useCallback(() => {
    setRangeDays(DEFAULT_REPORT_RANGE_DAYS);
  }, []);

  const exportExecutiveBrief = () => {
    const lines = [
      '# Fortuna executive posture brief',
      `Generated: ${new Date().toISOString()}`,
      `Time range: last ${activeRangeLabel}`,
      '',
      `Active findings: ${activeFindings ?? 'n/a'}`,
      `Critical findings: ${criticalFindings ?? 'n/a'}`,
      `Critical attack paths: ${criticalPaths ?? 'n/a'} / ${totalPaths ?? 'n/a'} total`,
      `Runtime exploited signals: ${exploitedSignals ?? 'n/a'}`,
      `Clusters in scope: ${fleetClusterCount ?? 'n/a'}`,
      `Pods inventoried: ${fleetPodCount ?? 'n/a'}`,
      `Open investigations: ${investigationStats.openCases}`,
      `Overdue remediation actions: ${investigationStats.overdueRemediation}`,
    ];
    const blob = new Blob([lines.join('\n')], { type: 'text/markdown;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `executive-posture-${new Date().toISOString().replace(/[:.]/g, '-')}.md`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const exportCsv = () => {
    const header = ['resource', 'action', 'count'];
    const rows = reports.map((r) => [r.resource || '', r.action || '', String(r.count ?? 0)]);
    const content = [header, ...rows].map((row) => row.map((v) => `"${String(v).replace(/"/g, '""')}"`).join(',')).join('\n');
    const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `audit-reports-${rangeDays}d-${new Date().toISOString().replace(/[:.]/g, '-')}.csv`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  };

  const exportRiskPosture = async (format: 'csv' | 'pdf') => {
    if (!canExportFindings) return;
    setRiskExporting(format);
    try {
      const params = { sinceMinutes: rangeToSinceMinutes(rangeDays) };
      if (format === 'csv') await api.exportRisksCSV(params);
      else await api.exportRisksPDF(params);
    } finally {
      setRiskExporting(null);
    }
  };

  if (!canViewReports) {
    return <Navigate to="/monitoring" replace />;
  }

  return (
    <PageLayout
      title={PAGE_TITLES.reports}
      description={`Posture exports, audit aggregates, and report-ready evidence for the last ${activeRangeLabel}.`}
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => navigate('/monitoring')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Monitoring
          </Button>
          <Button variant="secondary" size="sm" onClick={refreshAll} disabled={loading || postureLoading}>
            <RefreshCw className={`w-4 h-4 mr-2 ${loading || postureLoading ? 'animate-spin motion-reduce:animate-none' : ''}`} />
            Refresh
          </Button>
          <Button variant="secondary" size="sm" onClick={exportExecutiveBrief} disabled={postureLoading}>
            <Download className="w-4 h-4 mr-2" />
            Executive brief
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => exportRiskPosture('csv')}
            disabled={!canExportFindings || riskExporting != null}
            isLoading={riskExporting === 'csv'}
            title={canExportFindings ? 'Export risk findings as CSV' : 'Requires export.findings permission'}
          >
            <Download className="w-4 h-4 mr-2" />
            Risks CSV
          </Button>
        </div>
      }
    >
      <div className="grid gap-4">
      <section className="rounded-lg border border-border bg-surface/45 px-4 py-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            <h2 className="text-body font-semibold text-text">Report window</h2>
            <p className="mt-1 text-caption text-muted">
              Findings and audit aggregates are scoped to the last {activeRangeLabel}. Pipeline health and investigation counts are current snapshots.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="inline-flex rounded-lg border border-border bg-base/60 p-1" aria-label="Report time range">
              {REPORT_RANGE_OPTIONS.map((option) => (
                <button
                  key={option.days}
                  type="button"
                  onClick={() => setRangeDays(option.days)}
                  className={`rounded-md px-3 py-1.5 text-caption font-semibold transition-colors ${
                    rangeDays === option.days
                      ? 'bg-surface-2 text-text shadow-sm'
                      : 'text-muted hover:text-text'
                  }`}
                >
                  {option.label}
                </button>
              ))}
            </div>
            <Button
              variant="secondary"
              size="sm"
              onClick={resetRange}
              disabled={rangeDays === DEFAULT_REPORT_RANGE_DAYS}
              title="Reset report window to 1 day"
            >
              <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
              Reset
            </Button>
          </div>
        </div>
      </section>
      <section className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(20rem,0.38fr)]">
        <Card className="p-4">
          <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 className="fortuna-section-title">Executive posture</h2>
              <p className="mt-1 max-w-3xl text-caption text-muted">
                Findings use the selected time window. Pipeline health, inventory, and investigation metrics are current snapshots.
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="secondary" size="sm" onClick={() => exportRiskPosture('pdf')} disabled={!canExportFindings || riskExporting != null} isLoading={riskExporting === 'pdf'} title={canExportFindings ? 'Export risk findings as print-ready HTML' : 'Requires export.findings permission'}>
                <Download className="mr-1.5 h-3.5 w-3.5" />
                Risks PDF
              </Button>
            </div>
          </div>

          {postureError ? (
            <PageError
              title="Posture APIs unavailable"
              description={postureError}
              className="py-6"
              action={<Button variant="secondary" size="sm" onClick={refreshAll}>Retry</Button>}
            />
          ) : null}

          <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 xl:grid-cols-4">
        <ExecutiveMetricCard
          icon={<ShieldAlert className="w-4 h-4 text-critical" />}
          label="Critical exposure"
          value={postureLoading ? 'Loading' : criticalFindings ?? 'Unavailable'}
          detail={activeFindings != null ? `${activeFindings} active finding(s)` : 'Source unavailable'}
          onClick={() => navigate('/risks')}
        />
        <ExecutiveMetricCard
          icon={<Route className="w-4 h-4 text-warning" />}
          label="Critical attack paths"
          value={postureLoading ? 'Loading' : criticalPaths ?? 'Unavailable'}
          detail={totalPaths != null ? `${totalPaths} total path(s)` : 'Source unavailable'}
          onClick={() => navigate('/attack-paths')}
        />
        <ExecutiveMetricCard
          icon={<Activity className="w-4 h-4 text-brand" />}
          label="Runtime exploited signals"
          value={postureLoading ? 'Loading' : exploitedSignals ?? 'Unavailable'}
          detail="Current pipeline health layer 2"
          onClick={() => navigate('/monitoring')}
        />
        <ExecutiveMetricCard
          icon={<Layers className="w-4 h-4 text-info" />}
          label="Fleet inventory"
          value={postureLoading ? 'Loading' : fleetPodCount ?? 'Unavailable'}
          detail={
            fleetClusterCount != null
              ? `${fleetClusterCount} cluster(s)`
              : 'Current clusters / pods from stats API'
          }
          onClick={() => navigate('/resources')}
        />
          </div>
        </Card>

        <Card className="p-4">
          <h2 className="fortuna-card-title">Report readiness</h2>
          <div className="mt-3 space-y-3">
            <ReadinessRow
              icon={<ShieldCheck className="h-4 w-4 text-success" />}
              label="Posture snapshot"
              value={postureLoading ? 'Loading' : unavailablePostureSources.length === 0 ? 'Ready' : 'Partial'}
              detail={
                unavailablePostureSources.length === 0
                  ? 'All live posture sources loaded'
                  : `Missing ${unavailablePostureSources.join(', ')}`
              }
            />
            <ReadinessRow
              icon={<Download className="h-4 w-4 text-info" />}
              label="Risk exports"
              value={canExportFindings ? 'Enabled' : 'Restricted'}
              detail={canExportFindings ? 'CSV and print-ready PDF available' : 'Requires export.findings permission'}
            />
            <ReadinessRow
              icon={<Database className="h-4 w-4 text-warning" />}
              label="Audit aggregates"
              value={canPlatformAudit ? 'Enabled' : 'Restricted'}
              detail={canPlatformAudit ? `${reports.length} row(s), last ${activeRangeLabel}` : 'Requires system.audit.read permission'}
            />
          </div>
        </Card>
      </section>

      <section className="grid gap-3 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <Card className="p-4">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h2 className="fortuna-card-title">Operations summary</h2>
            <Button variant="ghost" size="sm" onClick={() => navigate('/investigation')} disabled={!canInvestigationsRead}>
              Open cases
            </Button>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
        {canInvestigationsRead ? (
        <ExecutiveMetricCard
          icon={<Briefcase className="w-4 h-4 text-success" />}
          label="Open investigations"
          value={investigationStats.openCases}
          detail={
            investigationStats.overdueRemediation > 0
              ? `${investigationStats.overdueRemediation} overdue remediation step(s)`
              : 'Server-persisted SOC cases'
          }
          onClick={() => navigate('/investigation')}
        />
        ) : null}
        <ExecutiveMetricCard
          icon={<ClipboardList className="w-4 h-4 text-muted" />}
          label="Audit activity"
          value={loading ? 'Loading' : auditTotal}
          detail={
            investigationStats.overdueRemediation > 0
              ? `${investigationStats.overdueRemediation} overdue remediation (investigations)`
              : topAuditAggregate
                ? `${topAuditAggregate.resource || 'resource'} / ${topAuditAggregate.action || 'action'}`
                : 'No aggregate yet'
          }
        />
          </div>
          <div className="mt-4 grid gap-2 text-caption text-muted-2">
            <p>Finding export permissions are enforced by `/api/v1/risk/insights/export`.</p>
            <p>Audit aggregates are generated from `audit_logs` for the selected report window.</p>
          </div>
        </Card>

        <Card className="p-4">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h2 className="fortuna-card-title">Finding distribution</h2>
            <Button variant="ghost" size="sm" onClick={() => navigate('/risks')}>
              Review risks
            </Button>
          </div>
          <div className="grid gap-2">
            <SeverityRow label="Critical" value={criticalFindings} severity="critical" loading={postureLoading} />
            <SeverityRow label="High" value={highFindings} severity="high" loading={postureLoading} />
            <SeverityRow label="Medium" value={mediumFindings} severity="medium" loading={postureLoading} />
            <SeverityRow label="Low" value={lowFindings} severity="low" loading={postureLoading} />
          </div>
        </Card>
      </section>

      <section className="grid gap-3 xl:grid-cols-[minmax(0,0.45fr)_minmax(0,0.55fr)]">
        <Card className="p-4">
          <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 className="fortuna-card-title">Audit overview</h2>
              <p className="mt-1 text-caption text-muted">Top resources and actions from audit logs in the last {activeRangeLabel}.</p>
            </div>
            <Button variant="secondary" size="sm" onClick={exportCsv} disabled={!canPlatformAudit || reports.length === 0 || loading} title={canPlatformAudit ? 'Export audit aggregate CSV' : 'Requires system.audit.read permission'}>
              <Download className="mr-1.5 h-3.5 w-3.5" />
              Audit CSV
            </Button>
          </div>
          {!canPlatformAudit ? (
            <AccessNotice
              title="Audit aggregates restricted"
              description="This section requires system.audit.read. Risk posture and report exports remain available according to your role."
            />
          ) : loading ? (
            <div className="rounded-lg border border-border/60 bg-base/40 p-4 text-caption text-muted">Loading audit aggregates...</div>
          ) : auditError ? (
            <PageError title="Could not load audit aggregates" description={auditError} className="py-6" action={<Button variant="secondary" size="sm" onClick={loadAuditReports}>Retry</Button>} />
          ) : reports.length === 0 ? (
            <PageEmpty title="No audit activity" description={`No audit aggregate data returned for the last ${activeRangeLabel}.`} className="py-6" />
          ) : (
            <div className="grid gap-4 md:grid-cols-2">
              <AuditBarList title="Top resources" items={auditByResource} max={maxAuditResourceCount} />
              <AuditBarList title="Top actions" items={auditByAction} max={maxAuditActionCount} />
            </div>
          )}
        </Card>

        <Card className="p-0 overflow-hidden">
          <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
            <div>
              <h2 className="fortuna-card-title">Audit aggregate table</h2>
              <p className="mt-1 text-caption text-muted">Resource/action counts from the audit log table, last {activeRangeLabel}.</p>
            </div>
            <BarChart3 className="h-4 w-4 text-muted" />
          </div>
          {!canPlatformAudit ? (
            <AccessNotice
              title="Table hidden by role"
              description="Backend authorization protects /api/v1/audit/reports with system.audit.read."
              className="m-4"
            />
          ) : loading ? (
            <div className="p-8 text-muted">Loading reports...</div>
          ) : auditError ? (
            <PageError title="Could not load reports" description={auditError} className="py-8" />
          ) : reports.length === 0 ? (
            <PageEmpty title="No reports available" description={`No audit aggregate data returned for the last ${activeRangeLabel}.`} className="py-8" />
          ) : (
            <div className="ui-table-scroll">
              <table className={UI_TABLE}>
                <thead className={UI_THEAD_STICKY}>
                  <tr>
                    <th className={UI_TH}>Resource</th>
                    <th className={UI_TH}>Action</th>
                    <th className={`${UI_TH} text-right`}>Count</th>
                  </tr>
                </thead>
                <tbody>
                  {reports.map((report) => (
                    <tr key={report.id} className={UI_TR}>
                      <td className={UI_TD}>
                        <div className="flex items-center font-medium text-text">
                          <FileText className="w-4 h-4 mr-3 text-muted shrink-0" />
                          {report.resource || 'unknown'}
                        </div>
                      </td>
                      <td className={`${UI_TD} text-muted`}>{report.action || 'unknown'}</td>
                      <td className={`${UI_TD} text-right text-text font-mono tabular-nums`}>{report.count ?? 0}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </section>
      </div>
    </PageLayout>
  );
};

const ExecutiveMetricCard: React.FC<{
  icon: React.ReactNode;
  label: string;
  value: string | number;
  detail: string;
  onClick?: () => void;
}> = ({ icon, label, value, detail, onClick }) => {
  const content = (
    <>
      <div className="flex items-center gap-2 text-caption text-muted uppercase tracking-wide">
        {icon}
        <span>{label}</span>
      </div>
      <div className="mt-2 text-2xl font-bold text-text tabular-nums">{value}</div>
      <div className="mt-1 text-caption text-muted">{detail}</div>
    </>
  );

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className="rounded-lg border border-border bg-surface/60 p-3 text-left hover:border-brand/60 hover:bg-surface transition-colors"
      >
        {content}
      </button>
    );
  }

  return <div className="rounded-lg border border-border bg-surface/60 p-3">{content}</div>;
};

const ReadinessRow: React.FC<{
  icon: React.ReactNode;
  label: string;
  value: string;
  detail: string;
}> = ({ icon, label, value, detail }) => (
  <div className="rounded-lg border border-border/70 bg-base/35 p-3">
    <div className="flex items-center justify-between gap-3">
      <div className="flex min-w-0 items-center gap-2">
        {icon}
        <span className="truncate text-caption font-semibold text-text">{label}</span>
      </div>
      <span className="shrink-0 rounded-md border border-border bg-surface px-2 py-0.5 text-meta font-semibold text-text">
        {value}
      </span>
    </div>
    <p className="mt-1 text-caption text-muted-2">{detail}</p>
  </div>
);

const SeverityRow: React.FC<{
  label: string;
  value: number | undefined;
  severity: 'critical' | 'high' | 'medium' | 'low';
  loading: boolean;
}> = ({ label, value, severity, loading }) => {
  const display = loading ? 'Loading' : value ?? 'Unavailable';
  const color =
    severity === 'critical'
      ? 'bg-critical'
      : severity === 'high'
        ? 'bg-high'
        : severity === 'medium'
          ? 'bg-medium'
          : 'bg-info';
  return (
    <div className="grid grid-cols-[7rem_minmax(0,1fr)_auto] items-center gap-3 rounded-lg border border-border/70 bg-base/35 px-3 py-2">
      <div className="flex items-center gap-2 text-caption font-semibold text-text">
        <span className={`h-2.5 w-2.5 rounded-full ${color}`} />
        {label}
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-surface-2">
        {typeof value === 'number' && value > 0 ? (
          <div className={`h-full rounded-full ${color}`} style={{ width: `${Math.min(100, Math.max(6, value))}%` }} />
        ) : null}
      </div>
      <div className="min-w-[4.5rem] text-right text-caption font-semibold tabular-nums text-text">{display}</div>
    </div>
  );
};

const AuditBarList: React.FC<{
  title: string;
  items: Array<[string, number]>;
  max: number;
}> = ({ title, items, max }) => (
  <div>
    <h3 className="mb-2 text-caption font-semibold text-text">{title}</h3>
    <div className="space-y-2">
      {items.map(([label, count]) => (
        <div key={label} className="grid gap-1">
          <div className="flex items-center justify-between gap-2 text-caption">
            <span className="min-w-0 truncate text-muted" title={label}>{label}</span>
            <span className="font-mono tabular-nums text-text">{count}</span>
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-surface-2">
            <div className="h-full rounded-full bg-brand" style={{ width: `${Math.max(4, Math.round((count / max) * 100))}%` }} />
          </div>
        </div>
      ))}
    </div>
  </div>
);

const AccessNotice: React.FC<{
  title: string;
  description: string;
  className?: string;
}> = ({ title, description, className = '' }) => (
  <div className={`rounded-lg border border-warning/30 bg-warning/10 p-4 text-caption ${className}`}>
    <div className="flex items-start gap-2">
      <AlertCircle className="mt-0.5 h-4 w-4 shrink-0 text-warning" />
      <div>
        <p className="font-semibold text-warning">{title}</p>
        <p className="mt-1 text-warning/80">{description}</p>
      </div>
    </div>
  </div>
);
