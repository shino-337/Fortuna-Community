import React from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowRight, Briefcase, Target } from 'lucide-react';
import { PageLayout } from '../../design-system/layouts/PageLayout';
import { PageLoading } from '../../design-system/components/PageStatus';
import { Button } from '../../components/ui/Button';
import { usePersonaDashboardData } from '../../hooks/usePersonaDashboardData';
import { RiskStatCards } from '../dashboard/RiskStatCards';
import { OperationalAbsenceSurface } from '../../design-system/OperationalAbsenceSurface';
import { DecisionSpine } from '../dashboard/DecisionSpine';
import { DashboardActionPanel } from '../dashboard/DashboardActionPanel';
import { PAGE_TITLES } from '../../lib/pageTitles';

const has = (sections: string[], id: string) => sections.includes(id);

export const ViewerDashboard: React.FC = () => {
  const navigate = useNavigate();
  const { sections, stats, insightsSummary, attackPathCount, coreReady, identityLabel } =
    usePersonaDashboardData();

  if (!coreReady) {
    return (
      <PageLayout title={PAGE_TITLES.homeViewer} description="Loading scoped exposure…">
        <PageLoading message="Loading scoped exposure…" className="min-h-[24rem]" />
      </PageLayout>
    );
  }

  const risksUrl = '/risks/findings';
  const criticalCount = Number(insightsSummary?.critical ?? stats.critical ?? 0);
  const affectedWorkloads = Number(stats.affectedPodCount ?? 0);

  return (
    <PageLayout
      title={PAGE_TITLES.homeViewer}
      description={`${identityLabel}: scoped threat narrative and assigned investigations.`}
      actions={
        <Button variant="secondary" onClick={() => navigate(risksUrl)}>
          Observed threats <ArrowRight size={14} className="ml-1" />
        </Button>
      }
    >
      <div className="space-y-5 p-4 sm:p-6">
        <DecisionSpine
          roleLabel="Platform owner"
          title={
            affectedWorkloads > 0
              ? `${affectedWorkloads} workload${affectedWorkloads === 1 ? '' : 's'} need exposure review`
              : 'No scoped workload exposure requiring action'
          }
          situation="Start from service exposure, then inspect the attack path and decide whether remediation or escalation is needed."
          impact={`${criticalCount} critical finding${criticalCount === 1 ? '' : 's'} and ${attackPathCount} attack path${attackPathCount === 1 ? '' : 's'} in ${stats.clusterName ?? 'the selected scope'}.`}
          owner={`${identityLabel}; remediation ownership follows the affected service team.`}
          actionLabel="Review observed threats"
          onAction={() => navigate(risksUrl)}
          secondaryAction={
            <Button type="button" variant="secondary" onClick={() => navigate('/reports')}>
              Open executive brief
            </Button>
          }
          steps={[
            { label: 'Service at risk', value: `${affectedWorkloads} workload${affectedWorkloads === 1 ? '' : 's'}`, detail: stats.clusterName ?? 'Current scope' },
            { label: 'Attack path', value: `${attackPathCount} path${attackPathCount === 1 ? '' : 's'}`, detail: 'Validate reachability' },
            { label: 'Impact', value: criticalCount > 0 ? 'Critical exposure' : 'Scoped exposure', detail: 'Map to service owner' },
            { label: 'Remediation', value: 'Review threat evidence', detail: 'Choose fix or exception' },
            { label: 'Status', value: 'Track in case', detail: 'Escalate if blocked' },
          ]}
          metrics={[
            { label: 'Critical', value: criticalCount, tone: criticalCount > 0 ? 'critical' : 'neutral' },
            { label: 'Workloads', value: affectedWorkloads, tone: affectedWorkloads > 0 ? 'high' : 'neutral' },
          ]}
        />

        {has(sections, 'exposure_summary') ? (
          <section className="space-y-3" aria-label="Supporting exposure evidence">
            <RiskStatCards
              insightsSummary={insightsSummary}
              statsCritical={stats.critical}
              clusterName={stats.clusterName}
            />
          </section>
        ) : null}

        {has(sections, 'threat_narrative') ? (
          <DashboardActionPanel
            icon={Target}
            title="Threat narrative"
            description="Prioritized exposure in the assigned scope. Open attack narratives for path evidence and reachability context."
            action={
              <Button variant="secondary" size="sm" onClick={() => navigate('/attack-paths')}>
                View attack narratives
              </Button>
            }
          />
        ) : null}

        {has(sections, 'assigned_investigations') ? (
          <DashboardActionPanel
            icon={Briefcase}
            title="Assigned investigations"
            description="Cases shared with you or owned by your team, with evidence and decision history preserved."
            action={
              <Button variant="secondary" size="sm" onClick={() => navigate('/investigation')}>
                Open investigations
              </Button>
            }
            tone="brand"
          />
        ) : null}

        {has(sections, 'scoped_trends') ? (
          <OperationalAbsenceSurface
            state="healthy_empty"
            reason="Trend charts appear when exposure signals exist in your time window."
          />
        ) : null}
      </div>
    </PageLayout>
  );
};
