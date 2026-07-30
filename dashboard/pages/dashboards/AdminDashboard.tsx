import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Activity, Boxes, GitBranch, Shield, ShieldAlert } from 'lucide-react';
import { PageLayout } from '../../design-system/layouts/PageLayout';
import { PageLoading } from '../../design-system/components/PageStatus';
import { Button } from '../../components/ui/Button';
import { usePersonaDashboardData } from '../../hooks/usePersonaDashboardData';
import { ScopedMetric } from '../../design-system/components/ScopedMetric';
import { useOperationalContext } from '../../hooks/useOperationalContext';
import { DecisionSpine } from '../dashboard/DecisionSpine';
import { DashboardActionPanel } from '../dashboard/DashboardActionPanel';
import { PAGE_TITLES } from '../../lib/pageTitles';

const has = (sections: string[], id: string) => sections.includes(id);

export const AdminDashboard: React.FC = () => {
  const navigate = useNavigate();
  const { sections, stats, insightsSummary, pipelineHealth, coreReady, identityLabel, exploitedCapCount, attackPathCount, partialErrors } =
    usePersonaDashboardData();
  const { ownership } = useOperationalContext();

  if (!coreReady) {
    return (
      <PageLayout title={PAGE_TITLES.homeAdmin} description="Loading oversight workspace…">
        <PageLoading message="Loading oversight workspace…" className="min-h-[24rem]" />
      </PageLayout>
    );
  }

  const activeFindingsCount = Number(insightsSummary?.total ?? pipelineHealth?.layer1?.insightCount ?? stats.insights ?? 0);
  const criticalFindingsCount = Number(insightsSummary?.critical ?? stats.critical ?? 0);
  const pipelineState = pipelineHealth?.layer1?.status ?? (partialErrors.length > 0 ? 'degraded' : 'unknown');
  const pipelineLabel = pipelineState === 'healthy'
    ? 'Healthy'
    : pipelineState === 'stale'
      ? 'Stale'
      : pipelineState === 'degraded'
        ? 'Degraded'
        : 'Unknown';

  return (
    <PageLayout
      title={PAGE_TITLES.homeAdmin}
      description={`${identityLabel}: telemetry reliability, governance, and operational oversight.`}
    >
      <div className="space-y-5 p-4 sm:p-6">
        <DecisionSpine
          roleLabel="Governance reviewer"
          title={
            exploitedCapCount > 0
              ? `${exploitedCapCount} exploited capabilit${exploitedCapCount === 1 ? 'y' : 'ies'} need oversight`
              : 'Platform integrity is ready for oversight review'
          }
          situation="Start with active findings and attack-path exposure, then validate telemetry freshness before accepting risk or routing work to owners."
          impact={`${activeFindingsCount} active finding${activeFindingsCount === 1 ? '' : 's'}, ${attackPathCount} attack path${attackPathCount === 1 ? '' : 's'}, and ${criticalFindingsCount} critical exposure${criticalFindingsCount === 1 ? '' : 's'} in scope.`}
          owner={`${identityLabel}; policy and acceptance ownership follows governance assignment.`}
          actionLabel="Review active findings"
          onAction={() => navigate('/risks/findings')}
          secondaryAction={
            <Button type="button" variant="secondary" onClick={() => navigate('/monitoring')}>
              Check telemetry
            </Button>
          }
          steps={[
            { label: 'Risk queue', value: `${activeFindingsCount} active`, detail: `${criticalFindingsCount} critical` },
            { label: 'Attack paths', value: `${attackPathCount} path${attackPathCount === 1 ? '' : 's'}`, detail: 'Validate reachability' },
            { label: 'Telemetry', value: pipelineLabel, detail: 'Check freshness first' },
            { label: 'Owner', value: identityLabel, detail: 'Route to accountable team' },
            { label: 'Governance', value: 'Record decision', detail: 'Preserve evidence' },
          ]}
          metrics={[
            { label: 'Findings', value: activeFindingsCount, tone: criticalFindingsCount > 0 ? 'critical' : activeFindingsCount > 0 ? 'medium' : 'neutral' },
            { label: 'Attack paths', value: attackPathCount, tone: attackPathCount > 0 ? 'high' : 'neutral' },
            { label: 'Capabilities', value: exploitedCapCount, tone: exploitedCapCount > 0 ? 'critical' : 'neutral' },
          ]}
        />

        {has(sections, 'telemetry_degradation') || has(sections, 'telemetry_reliability') ? (
          <section className="grid gap-3 sm:grid-cols-3">
            <ScopedMetric label="Findings in pipeline" value={pipelineHealth?.layer1?.insightCount} ownership={ownership} />
            <ScopedMetric label="Exploited capabilities" value={exploitedCapCount} ownership={ownership} />
            <ScopedMetric label="Attack paths in scope" value={attackPathCount} ownership={ownership} />
          </section>
        ) : null}

        {has(sections, 'platform_integrity') ? (
          <DashboardActionPanel
            icon={ShieldAlert}
            title="Active risk queue"
            description={`Open findings requiring triage or acceptance. ${criticalFindingsCount} critical exposure${criticalFindingsCount === 1 ? '' : 's'} and ${activeFindingsCount} total active finding${activeFindingsCount === 1 ? '' : 's'} are in the current scope.`}
            action={
              <Button variant="secondary" onClick={() => navigate('/risks/findings')}>
                Open findings
              </Button>
            }
            tone={criticalFindingsCount > 0 ? 'warning' : 'neutral'}
          />
        ) : null}

        {has(sections, 'attack_chains') ? (
          <DashboardActionPanel
            icon={GitBranch}
            title="Attack-path exposure"
            description="Reachability paths that connect entry resources to higher-impact resources. Use this to confirm blast radius before prioritizing remediation."
            action={
              <Button variant="secondary" onClick={() => navigate('/attack-paths')}>
                Review attack paths
              </Button>
            }
            tone={attackPathCount > 0 ? 'warning' : 'neutral'}
          />
        ) : null}

        {has(sections, 'governance_exposure') ? (
          <DashboardActionPanel
            icon={Shield}
            title="Audit and governance"
            description="Security activity, access review, exceptions, and acceptance decisions. Use this after findings are validated, not as the primary finding queue."
            action={
              <Button variant="secondary" onClick={() => navigate('/governance')}>
                Open governance
              </Button>
            }
            tone="brand"
          />
        ) : null}

        {has(sections, 'operational_oversight') || has(sections, 'platform_integrity') ? (
          <DashboardActionPanel
            icon={Activity}
            title="Pipeline and runtime health"
            description={`Pipeline state: ${pipelineLabel}. Clusters: ${stats.clusters}. Review telemetry health before acting on stale or incomplete signals.`}
            action={
              <Button variant="secondary" onClick={() => navigate('/monitoring')}>
                Open telemetry
              </Button>
            }
          />
        ) : null}

        {has(sections, 'cluster_posture') ? (
          <DashboardActionPanel
            icon={Boxes}
            title="Cluster posture"
            description="Review inventory health, ownership, and cluster-level exposure for the current operational scope."
            action={
              <Button variant="secondary" onClick={() => navigate('/clusters')}>
                Review cluster posture
              </Button>
            }
          />
        ) : null}
      </div>
    </PageLayout>
  );
};
