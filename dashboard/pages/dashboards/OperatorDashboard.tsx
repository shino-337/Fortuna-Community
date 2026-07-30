import React from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowRight, AlertTriangle, Briefcase } from 'lucide-react';
import { PageLayout } from '../../design-system/layouts/PageLayout';
import { PageLoading } from '../../design-system/components/PageStatus';
import { Button } from '../../components/ui/Button';
import { usePersonaDashboardData } from '../../hooks/usePersonaDashboardData';
import { RiskStatCards } from '../dashboard/RiskStatCards';
import { IncidentPriorityStrip } from '../../components/IncidentPriorityStrip';
import { DecisionSpine } from '../dashboard/DecisionSpine';
import { DashboardActionPanel } from '../dashboard/DashboardActionPanel';
import { PAGE_TITLES } from '../../lib/pageTitles';

const has = (sections: string[], id: string) => sections.includes(id);

export const OperatorDashboard: React.FC = () => {
  const navigate = useNavigate();
  const {
    sections,
    stats,
    insightsSummary,
    attackPathCount,
    coreReady,
    identityLabel,
    partialErrors,
  } = usePersonaDashboardData();

  if (!coreReady) {
    return (
      <PageLayout title={PAGE_TITLES.homeOperator} description="Loading responder workspace…">
        <PageLoading message="Loading responder workspace…" className="min-h-[24rem]" />
      </PageLayout>
    );
  }

  const criticalCount = Number(insightsSummary?.critical ?? stats.critical ?? 0);
  const affectedWorkloads = Number(stats.affectedPodCount ?? 0);
  const totalFindings = Number(insightsSummary?.total ?? 0);
  const primaryAction = criticalCount > 0 || totalFindings > 0 ? 'Open threat operations' : 'Review investigations';

  return (
    <PageLayout
      title={PAGE_TITLES.homeOperator}
      description={`${identityLabel}: triage, investigation continuity, and remediation flow.`}
      actions={
        <Button variant="secondary" onClick={() => navigate('/investigation')}>
          Investigations <ArrowRight size={14} className="ml-1" />
        </Button>
      }
    >
      <div className="space-y-5 p-4 sm:p-6">
        <DecisionSpine
          roleLabel="SOC analyst"
          title={
            criticalCount > 0
              ? `${criticalCount} critical finding${criticalCount === 1 ? '' : 's'} require triage`
              : attackPathCount > 0
                ? `${attackPathCount} attack path${attackPathCount === 1 ? '' : 's'} need review`
                : 'No critical queue pressure in scope'
          }
          situation={
            partialErrors.length > 0
              ? 'Telemetry is degraded, so prioritize confirmed findings and keep investigation context visible.'
              : 'Use the current scope to move from top risk to evidence, affected assets, and action.'
          }
          impact={`${affectedWorkloads} workload${affectedWorkloads === 1 ? '' : 's'} affected in ${stats.clusterName ?? 'the selected scope'}.`}
          owner={`${identityLabel}; response ownership follows active case assignment.`}
          actionLabel={primaryAction}
          onAction={() => navigate(totalFindings > 0 ? '/risks/findings' : '/investigation')}
          secondaryAction={
            <Button type="button" variant="secondary" onClick={() => navigate('/attack-paths')}>
              Inspect attack paths
            </Button>
          }
          steps={[
            { label: 'Top risk', value: criticalCount > 0 ? 'Critical findings' : 'Attack path review', detail: `${totalFindings} open finding${totalFindings === 1 ? '' : 's'}` },
            { label: 'Evidence', value: partialErrors.length > 0 ? 'Telemetry degraded' : 'Runtime + graph signals', detail: partialErrors.length > 0 ? 'Verify confidence' : 'Correlate first' },
            { label: 'Affected assets', value: `${affectedWorkloads} workload${affectedWorkloads === 1 ? '' : 's'}`, detail: stats.clusterName ?? 'Current scope' },
            { label: 'Required action', value: primaryAction, detail: 'Start with actionable items' },
            { label: 'Escalate', value: 'Investigation case', detail: 'Pin evidence and owner' },
          ]}
          metrics={[
            { label: 'Critical', value: criticalCount, tone: criticalCount > 0 ? 'critical' : 'neutral' },
            { label: 'Paths', value: attackPathCount, tone: attackPathCount > 0 ? 'high' : 'neutral' },
          ]}
        />
        <IncidentPriorityStrip
          criticalCount={criticalCount}
          attackPathCount={attackPathCount}
          hasOpenFindings={totalFindings > 0}
          telemetryDegraded={partialErrors.length > 0}
        />

        {has(sections, 'triage_queue') ? (
          <section className="space-y-3" aria-label="Supporting triage evidence">
            <RiskStatCards
              insightsSummary={insightsSummary}
              statsCritical={stats.critical}
              clusterName={stats.clusterName}
            />
          </section>
        ) : null}

        {has(sections, 'active_investigations') ? (
          <DashboardActionPanel
            icon={Briefcase}
            title="Active investigations"
            description="Continue pinned evidence and decision log in the investigation workspace."
            action={
              <Button variant="secondary" size="sm" onClick={() => navigate('/investigation')}>
                Resume investigation
              </Button>
            }
            tone="warning"
          />
        ) : null}

        {has(sections, 'remediation_blockers') ? (
          <DashboardActionPanel
            icon={AlertTriangle}
            title="Remediation blockers"
            description="Blocked remediation steps surface here when a case needs ownership, an exception, or escalation."
            tone="neutral"
          />
        ) : null}
      </div>
    </PageLayout>
  );
};
