import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../../lib/api';
import { can, P } from '../../lib/permissions';
import { usePermUser } from '../../hooks/usePermUser';
import { Card } from '../../design-system/components/Card';
import { Download, Activity, Route, ShieldAlert, Layers, Briefcase, RefreshCw, ShieldCheck, FileText } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { PageError } from '../../design-system/components/PageStatus';
import { ResetFiltersButton } from '../../components/ResetFiltersButton';
import { downloadText } from '../../lib/download';
import { useToast } from '../../design-system/components/Toast';
import { useClusterStore } from '../../store/clusterStore';

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

/**
 * Executive brief on Home (formerly the Reports page). It stays collapsed until opened and
 * only then loads its own report window, so Home does not pay for it on every visit.
 */
export const ExecutiveBriefPanel: React.FC<{ open: boolean; onToggle: (open: boolean) => void }> = ({ open, onToggle }) => {
  const navigate = useNavigate();
  const toast = useToast();
  const permUser = usePermUser();
  const canFindingsRead = can(permUser, P.findingsRead);
  const canPipelineHealth = can(permUser, P.observabilityMetricsRead);
  const canExportFindings = can(permUser, P.exportFindings);
  const canInvestigationsRead = can(permUser, P.investigationsRead);
  const [rangeDays, setRangeDays] = useState<ReportRangeDays>(DEFAULT_REPORT_RANGE_DAYS);
  const [postureLoading, setPostureLoading] = useState(false);
  const [postureLoaded, setPostureLoaded] = useState(false);
  const [postureError, setPostureError] = useState<string | null>(null);
  const [riskExporting, setRiskExporting] = useState<'csv' | 'pdf' | null>(null);
  const [posture, setPosture] = useState<ExecutivePosture>({ stats: null, summary: null, pipeline: null });
  // null = not loaded (no permission or the request failed); never shown as 0.
  const [investigationStats, setInvestigationStats] = useState<{ openCases: number; overdueRemediation: number } | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  // Findings and inventory follow the header cluster like the rest of Home; pipeline and case counts are platform-wide.
  const clusterId = useClusterStore((st) => st.selectedClusterId) || null;

  useEffect(() => {
    if (!open || !canFindingsRead) return;
    let cancelled = false;
    setPostureLoading(true);
    setPostureError(null);
    const sinceMinutes = rangeToSinceMinutes(rangeDays);
    Promise.allSettled([
      api.getStats(clusterId),
      api.getInsightsSummary(clusterId, sinceMinutes),
      canPipelineHealth ? api.getPipelineHealth() : Promise.reject(new Error('observability.metrics.read required')),
      canInvestigationsRead ? api.getInvestigationCaseStats() : Promise.reject(new Error('investigations.read required')),
    ]).then(([statsResult, summaryResult, pipelineResult, investigationResult]) => {
      if (cancelled) return;
      setPosture({
        stats: statsResult.status === 'fulfilled' ? statsResult.value : null,
        summary: summaryResult.status === 'fulfilled' ? summaryResult.value : null,
        pipeline: pipelineResult.status === 'fulfilled' ? pipelineResult.value : null,
      });
      setInvestigationStats(investigationResult.status === 'fulfilled' ? investigationResult.value : null);
      // Only show error if key APIs failed (stats + summary)
      if (statsResult.status === 'rejected' && summaryResult.status === 'rejected') {
        setPostureError('Could not load posture APIs.');
      }
      setPostureLoaded(true);
    }).finally(() => {
      if (!cancelled) setPostureLoading(false);
    });
    return () => {
      cancelled = true;
    };
  }, [open, canFindingsRead, canPipelineHealth, canInvestigationsRead, rangeDays, reloadKey, clusterId]);

  if (!canFindingsRead) return null;

  const busy = postureLoading || !postureLoaded;
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

  const refresh = () => setReloadKey((k) => k + 1);

  const exportExecutiveBrief = () => {
    const lines = [
      '# Fortuna executive posture brief',
      `Generated: ${new Date().toISOString()}`,
      `Time range: last ${activeRangeLabel}`,
      `Cluster scope: ${clusterId ?? 'all clusters in your scope'}`,
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
      const params = { clusterId, sinceMinutes: rangeToSinceMinutes(rangeDays) };
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

  return (
    <details
      id="executive-brief"
      open={open}
      onToggle={(event) => {
        const next = (event.currentTarget as HTMLDetailsElement).open;
        if (next !== open) onToggle(next);
      }}
      className="group scroll-mt-24 overflow-hidden rounded-xl border border-border/80 bg-surface/30"
    >
      <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-4 py-3 text-caption font-semibold text-text hover:bg-surface-2/40 [&::-webkit-details-marker]:hidden">
        <span className="inline-flex items-center gap-2">
          <FileText className="h-4 w-4 text-brand" />
          Executive brief
        </span>
        <span className="font-normal text-meta text-muted-2">Posture summary and risk exports</span>
      </summary>
      {open ? (
      <div className="grid gap-4 border-t border-border/80 p-4">
      <section className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <p className="min-w-0 text-caption text-muted">
          Findings cover the last {activeRangeLabel} in the header cluster scope. Pipeline health, inventory, and investigation counts are current snapshots.
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex rounded-lg border border-border bg-base/60 p-1" role="group" aria-label="Report time range">
            {REPORT_RANGE_OPTIONS.map((option) => (
              <button
                key={option.days}
                type="button"
                aria-pressed={rangeDays === option.days}
                onClick={() => setRangeDays(option.days)}
                className={`rounded-md px-3 py-1.5 text-caption font-semibold transition-colors ${
                  rangeDays === option.days ? 'bg-surface-2 text-text shadow-sm' : 'text-muted hover:text-text'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
          <ResetFiltersButton
            onReset={() => setRangeDays(DEFAULT_REPORT_RANGE_DAYS)}
            active={rangeDays !== DEFAULT_REPORT_RANGE_DAYS}
            title="Reset report window to 1 day"
          />
          <Button variant="secondary" size="sm" onClick={refresh} disabled={postureLoading}>
            <RefreshCw className={`mr-1.5 h-3.5 w-3.5 ${postureLoading ? 'animate-spin motion-reduce:animate-none' : ''}`} />
            Refresh
          </Button>
          <Button variant="secondary" size="sm" onClick={exportExecutiveBrief} disabled={busy}>
            <Download className="mr-1.5 h-3.5 w-3.5" />
            Download brief
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => exportRiskPosture('csv')}
            disabled={!canExportFindings || riskExporting != null}
            isLoading={riskExporting === 'csv'}
            title={canExportFindings ? 'Export risk findings as CSV' : 'Requires export.findings permission'}
          >
            <Download className="mr-1.5 h-3.5 w-3.5" />
            Risks CSV
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => exportRiskPosture('pdf')}
            disabled={!canExportFindings || riskExporting != null}
            isLoading={riskExporting === 'pdf'}
            title={canExportFindings ? 'Export risk findings as print-ready HTML' : 'Requires export.findings permission'}
          >
            <Download className="mr-1.5 h-3.5 w-3.5" />
            Risks PDF
          </Button>
        </div>
      </section>

      {postureError ? (
        <PageError
          title="Posture APIs unavailable"
          description={postureError}
          className="py-6"
          action={<Button variant="secondary" size="sm" onClick={refresh}>Retry</Button>}
        />
      ) : null}

      <section className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(20rem,0.38fr)]">
        <Card className="p-4">
          <h3 className="fortuna-card-title mb-3">Executive posture</h3>
          <div className="grid grid-cols-1 gap-3 min-[520px]:grid-cols-2 xl:grid-cols-4">
            <ExecutiveMetricCard
              icon={<ShieldAlert className="w-4 h-4 text-critical" />}
              label="Critical exposure"
              value={busy ? 'Loading' : criticalFindings ?? 'Unavailable'}
              detail={activeFindings != null ? `${activeFindings} active finding(s)` : 'Source unavailable'}
              onClick={() => navigate('/risks')}
            />
            <ExecutiveMetricCard
              icon={<Route className="w-4 h-4 text-warning" />}
              label="Critical attack paths"
              value={busy ? 'Loading' : criticalPaths ?? 'Unavailable'}
              detail={totalPaths != null ? `${totalPaths} total path(s)` : 'Source unavailable'}
              onClick={() => navigate('/attack-paths')}
            />
            <ExecutiveMetricCard
              icon={<Activity className="w-4 h-4 text-brand" />}
              label="Runtime exploited signals"
              value={busy ? 'Loading' : exploitedSignals ?? 'Unavailable'}
              detail="Current pipeline health layer 2"
              onClick={() => navigate('/monitoring')}
            />
            <ExecutiveMetricCard
              icon={<Layers className="w-4 h-4 text-info" />}
              label="Fleet inventory"
              value={busy ? 'Loading' : fleetPodCount ?? 'Unavailable'}
              detail={fleetClusterCount != null ? `${fleetClusterCount} cluster(s)` : 'Current clusters / pods from stats API'}
              onClick={() => navigate('/resources')}
            />
          </div>
        </Card>

        <Card className="p-4">
          <h3 className="fortuna-card-title">Report readiness</h3>
          <div className="mt-3 space-y-3">
            <ReadinessRow
              icon={<ShieldCheck className="h-4 w-4 text-success" />}
              label="Posture snapshot"
              value={busy ? 'Loading' : unavailablePostureSources.length === 0 ? 'Ready' : 'Partial'}
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
        {canInvestigationsRead ? (
          <Card className="p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <h3 className="fortuna-card-title">Operations summary</h3>
              <Button variant="ghost" size="sm" onClick={() => navigate('/investigation')}>
                Open cases
              </Button>
            </div>
            <ExecutiveMetricCard
              icon={<Briefcase className="w-4 h-4 text-success" />}
              label="Open investigations"
              value={busy ? 'Loading' : investigationStats ? investigationStats.openCases : 'n/a'}
              detail={
                !investigationStats
                  ? 'Investigation stats unavailable'
                  : investigationStats.overdueRemediation > 0
                    ? `${investigationStats.overdueRemediation} overdue remediation step(s)`
                    : 'Server-persisted SOC cases'
              }
              onClick={() => navigate('/investigation')}
            />
          </Card>
        ) : null}

        <Card className="p-4">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="fortuna-card-title">Finding distribution</h3>
            <Button variant="ghost" size="sm" onClick={() => navigate('/risks')}>
              Review risks
            </Button>
          </div>
          <div className="grid gap-2">
            <SeverityRow label="Critical" value={criticalFindings} severity="critical" total={activeFindings} loading={busy} />
            <SeverityRow label="High" value={highFindings} severity="high" total={activeFindings} loading={busy} />
            <SeverityRow label="Medium" value={mediumFindings} severity="medium" total={activeFindings} loading={busy} />
            <SeverityRow label="Low" value={lowFindings} severity="low" total={activeFindings} loading={busy} />
          </div>
        </Card>
      </section>
      </div>
      ) : null}
    </details>
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
  /** All active findings; the bar shows this level's share of them. */
  total: number | undefined;
  loading: boolean;
}> = ({ label, value, severity, total, loading }) => {
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
        {typeof value === 'number' && value > 0 && typeof total === 'number' && total > 0 ? (
          <div className={`h-full rounded-full ${color}`} style={{ width: `${Math.min(100, Math.max(6, (value / total) * 100))}%` }} />
        ) : null}
      </div>
      <div className="min-w-[4.5rem] text-right text-caption font-semibold tabular-nums text-text">{display}</div>
    </div>
  );
};

