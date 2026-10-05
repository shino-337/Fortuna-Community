import React, { useCallback, useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { can, P } from '../lib/permissions';
import { usePermUser } from '../hooks/usePermUser';
import { Card } from '../design-system/components/Card';
import {
  Download,
  ArrowLeft,
  Activity,
  Route,
  ShieldAlert,
  Layers,
  Briefcase,
  RefreshCw,
  ShieldCheck,
  RotateCcw,
} from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageError } from '../design-system/components/PageStatus';
import { PAGE_TITLES } from '../lib/pageTitles';
import { downloadText } from '../lib/download';
import { useToast } from '../design-system/components/Toast';

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

function rangeToSinceMinutes(days: ReportRangeDays): number {
  return days * 24 * 60;
}

export const Reports: React.FC = () => {
  const navigate = useNavigate();
  const toast = useToast();
  const permUser = usePermUser();
  const canFindingsRead = can(permUser, P.findingsRead);
  const canPipelineHealth = can(permUser, P.observabilityMetricsRead);
  const canExportFindings = can(permUser, P.exportFindings);
  const [rangeDays, setRangeDays] = useState<ReportRangeDays>(DEFAULT_REPORT_RANGE_DAYS);
  const [postureLoading, setPostureLoading] = useState(true);
  const [postureError, setPostureError] = useState<string | null>(null);
  const [riskExporting, setRiskExporting] = useState<'csv' | 'pdf' | null>(null);
  const [posture, setPosture] = useState<ExecutivePosture>({ stats: null, summary: null, pipeline: null });
  const canInvestigationsRead = can(permUser, P.investigationsRead);
  // null = not loaded (no permission or the request failed); never shown as 0.
  const [investigationStats, setInvestigationStats] = useState<{ openCases: number; overdueRemediation: number } | null>(null);

  const loadPosture = useCallback(() => {
    if (!canFindingsRead) {
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
      canPipelineHealth ? api.getPipelineHealth() : Promise.reject(new Error('observability.metrics.read required')),
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
  }, [canFindingsRead, canPipelineHealth, rangeDays]);

  useEffect(() => {
    if (!canInvestigationsRead) {
      setInvestigationStats(null);
      return;
    }
    let cancelled = false;
    api.getInvestigationCaseStats()
      .then((stats) => {
        if (!cancelled) setInvestigationStats(stats);
      })
      .catch(() => {
        if (!cancelled) setInvestigationStats(null);
      });
    return () => {
      cancelled = true;
    };
  }, [canInvestigationsRead]);

  useEffect(() => {
    return loadPosture();
  }, [loadPosture]);

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
  const activeRangeLabel = `${rangeDays} day${rangeDays === 1 ? '' : 's'}`;

  const refreshAll = useCallback(() => {
    loadPosture();
  }, [loadPosture]);

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
      `Open investigations: ${investigationStats?.openCases ?? 'n/a'}`,
      `Overdue remediation actions: ${investigationStats?.overdueRemediation ?? 'n/a'}`,
    ];
    downloadText(lines.join('\n'), `executive-posture-${new Date().toISOString().replace(/[:.]/g, '-')}.md`, 'text/markdown;charset=utf-8');
  };

  const exportRiskPosture = async (format: 'csv' | 'pdf') => {
    if (!canExportFindings) return;
    setRiskExporting(format);
    try {
      const params = { sinceMinutes: rangeToSinceMinutes(rangeDays) };
      if (format === 'csv') await api.exportRisksCSV(params);
      else await api.exportRisksPDF(params);
    } catch (err) {
      toast({
        title: 'Export failed',
        description: err instanceof Error ? err.message : 'The findings export could not be downloaded.',
        variant: 'error',
      });
    } finally {
      setRiskExporting(null);
    }
  };

  if (!canFindingsRead) {
    return <Navigate to="/monitoring" replace />;
  }

  return (
    <PageLayout
      title={PAGE_TITLES.reports}
      description={`Posture exports and report-ready evidence for the last ${activeRangeLabel}. Audit summaries are on the Audit page.`}
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => (window.history.length > 1 ? navigate(-1) : navigate('/'))}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back
          </Button>
          <Button variant="secondary" size="sm" onClick={refreshAll} disabled={postureLoading}>
            <RefreshCw className={`w-4 h-4 mr-2 ${postureLoading ? 'animate-spin motion-reduce:animate-none' : ''}`} />
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
              Findings are scoped to the last {activeRangeLabel}. Pipeline health and investigation counts are current snapshots.
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
          value={investigationStats ? investigationStats.openCases : 'n/a'}
          detail={
            !investigationStats
              ? 'Investigation stats unavailable'
              : investigationStats.overdueRemediation > 0
                ? `${investigationStats.overdueRemediation} overdue remediation step(s)`
                : 'Server-persisted SOC cases'
          }
          onClick={() => navigate('/investigation')}
        />
        ) : null}
          </div>
          <div className="mt-4 grid gap-2 text-caption text-muted-2">
            <p>Finding export permissions are enforced by `/api/v1/risk/insights/export`.</p>
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

