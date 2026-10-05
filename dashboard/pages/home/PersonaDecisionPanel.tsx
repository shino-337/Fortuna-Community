import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Activity, AlertTriangle, Boxes, Briefcase, GitBranch, Shield, Target } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { DecisionSpine } from '../dashboard/DecisionSpine';
import { DashboardActionPanel } from '../dashboard/DashboardActionPanel';
import type { ShellVariant } from '../../lib/operationalSurfaceRegistry';

/** Counts Home already loaded; the panel never fetches on its own. */
export interface PersonaDecisionData {
  activeFindings: number;
  criticalFindings: number;
  attackPathCount: number;
  exploitedCapCount: number;
  affectedWorkloads: number;
  clusterName?: string | null;
  clusterCount: number;
  /** Layer 1 pipeline status, or null when the pipeline health API was not loaded. */
  pipelineStatus: string | null;
  telemetryDegraded: boolean;
}

interface PersonaDecisionPanelProps {
  shellVariant: ShellVariant;
  sections: string[];
  identityLabel: string;
  data: PersonaDecisionData;
  onOpenBrief?: () => void;
}

const plural = (n: number, one: string, many = `${one}s`) => `${n} ${n === 1 ? one : many}`;

/** Persona-specific decision summary shown at the top of Home. */
export const PersonaDecisionPanel: React.FC<PersonaDecisionPanelProps> = ({ shellVariant, ...props }) => {
  switch (shellVariant) {
    case 'admin':
      return <AdminDecisionPanel {...props} />;
    case 'operator':
      return <OperatorDecisionPanel {...props} />;
    case 'viewer':
      return <ViewerDecisionPanel {...props} />;
    default:
      return null;
  }
};

type PanelProps = Omit<PersonaDecisionPanelProps, 'shellVariant'>;

const AdminDecisionPanel: React.FC<PanelProps> = ({ sections, identityLabel, data }) => {
  const navigate = useNavigate();
  const { activeFindings, criticalFindings, attackPathCount, exploitedCapCount } = data;
  const pipelineState = data.pipelineStatus ?? (data.telemetryDegraded ? 'degraded' : 'unknown');
  const pipelineLabel =
    pipelineState === 'healthy' ? 'Healthy' : pipelineState === 'stale' ? 'Stale' : pipelineState === 'degraded' ? 'Degraded' : 'Unknown';

  return (
    <div className="space-y-4">
      <DecisionSpine
        roleLabel="Governance reviewer"
        title={
          exploitedCapCount > 0
            ? `${exploitedCapCount} exploited capabilit${exploitedCapCount === 1 ? 'y' : 'ies'} need oversight`
            : 'Platform integrity is ready for oversight review'
        }
        situation="Start with active findings and attack-path exposure, then validate telemetry freshness before accepting risk or routing work to owners."
        impact={`${plural(activeFindings, 'active finding')}, ${plural(attackPathCount, 'attack path')}, and ${plural(criticalFindings, 'critical exposure')} in scope.`}
        owner={`${identityLabel}; policy and acceptance ownership follows governance assignment.`}
        actionLabel="Review active findings"
        onAction={() => navigate('/risks/findings')}
        secondaryAction={
          <Button type="button" variant="secondary" onClick={() => navigate('/monitoring')}>
            Check telemetry
          </Button>
        }
        steps={[
          { label: 'Risk queue', value: `${activeFindings} active`, detail: `${criticalFindings} critical` },
          { label: 'Attack paths', value: plural(attackPathCount, 'path'), detail: 'Validate reachability' },
          { label: 'Telemetry', value: pipelineLabel, detail: 'Check freshness first' },
          { label: 'Owner', value: identityLabel, detail: 'Route to accountable team' },
          { label: 'Audit', value: 'Record decision', detail: 'Preserve evidence' },
        ]}
        metrics={[
          { label: 'Findings', value: activeFindings, tone: criticalFindings > 0 ? 'critical' : activeFindings > 0 ? 'medium' : 'neutral' },
          { label: 'Attack paths', value: attackPathCount, tone: attackPathCount > 0 ? 'high' : 'neutral' },
          { label: 'Capabilities', value: exploitedCapCount, tone: exploitedCapCount > 0 ? 'critical' : 'neutral' },
        ]}
      />
      <div className="grid gap-3 lg:grid-cols-2">
        {sections.includes('governance_exposure') ? (
          <DashboardActionPanel
            icon={Shield}
            title="Audit and governance"
            description="Security activity, access review, exceptions, and acceptance decisions. Use this after findings are validated, not as the primary finding queue."
            action={
              <Button variant="secondary" onClick={() => navigate('/governance')}>
                Open audit
              </Button>
            }
            tone="brand"
          />
        ) : null}
        {sections.includes('operational_oversight') || sections.includes('platform_integrity') ? (
          <DashboardActionPanel
            icon={Activity}
            title="Pipeline and runtime health"
            description={`Pipeline state: ${pipelineLabel}. Clusters: ${data.clusterCount}. Review telemetry health before acting on stale or incomplete signals.`}
            action={
              <Button variant="secondary" onClick={() => navigate('/monitoring')}>
                Open telemetry
              </Button>
            }
          />
        ) : null}
        {sections.includes('cluster_posture') ? (
          <DashboardActionPanel
            icon={Boxes}
            title="Cluster posture"
            description="Review inventory health, ownership, and cluster-level exposure for the current operational scope."
            action={
              <Button variant="secondary" onClick={() => navigate('/resources/clusters')}>
                Review cluster posture
              </Button>
            }
          />
        ) : null}
        {sections.includes('attack_chains') ? (
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
      </div>
    </div>
  );
};

const OperatorDecisionPanel: React.FC<PanelProps> = ({ sections, identityLabel, data }) => {
  const navigate = useNavigate();
  const { activeFindings, criticalFindings, attackPathCount, affectedWorkloads, telemetryDegraded } = data;
  const scopeName = data.clusterName ?? 'the selected scope';
  const primaryAction = activeFindings > 0 ? 'Open threat operations' : 'Review investigations';

  return (
    <div className="space-y-4">
      <DecisionSpine
        roleLabel="SOC analyst"
        title={
          criticalFindings > 0
            ? `${plural(criticalFindings, 'critical finding')} require triage`
            : attackPathCount > 0
              ? `${plural(attackPathCount, 'attack path')} need review`
              : 'No critical queue pressure in scope'
        }
        situation={
          telemetryDegraded
            ? 'Telemetry is degraded, so prioritize confirmed findings and keep investigation context visible.'
            : 'Use the current scope to move from top risk to evidence, affected assets, and action.'
        }
        impact={`${plural(affectedWorkloads, 'workload')} affected in ${scopeName}.`}
        owner={`${identityLabel}; response ownership follows active case assignment.`}
        actionLabel={primaryAction}
        onAction={() => navigate(activeFindings > 0 ? '/risks/findings' : '/investigation')}
        secondaryAction={
          <Button type="button" variant="secondary" onClick={() => navigate('/attack-paths')}>
            Inspect attack paths
          </Button>
        }
        steps={[
          { label: 'Top risk', value: criticalFindings > 0 ? 'Critical findings' : 'Attack path review', detail: plural(activeFindings, 'open finding') },
          { label: 'Evidence', value: telemetryDegraded ? 'Telemetry degraded' : 'Runtime + graph signals', detail: telemetryDegraded ? 'Verify confidence' : 'Correlate first' },
          { label: 'Affected assets', value: plural(affectedWorkloads, 'workload'), detail: data.clusterName ?? 'Current scope' },
          { label: 'Required action', value: primaryAction, detail: 'Start with actionable items' },
          { label: 'Escalate', value: 'Investigation case', detail: 'Pin evidence and owner' },
        ]}
        metrics={[
          { label: 'Critical', value: criticalFindings, tone: criticalFindings > 0 ? 'critical' : 'neutral' },
          { label: 'Paths', value: attackPathCount, tone: attackPathCount > 0 ? 'high' : 'neutral' },
        ]}
      />
      <div className="grid gap-3 lg:grid-cols-2">
        {sections.includes('active_investigations') ? (
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
        {sections.includes('remediation_blockers') ? (
          <DashboardActionPanel
            icon={AlertTriangle}
            title="Remediation blockers"
            description="Blocked remediation steps surface here when a case needs ownership, an exception, or escalation."
            tone="neutral"
          />
        ) : null}
      </div>
    </div>
  );
};

const ViewerDecisionPanel: React.FC<PanelProps> = ({ sections, identityLabel, data, onOpenBrief }) => {
  const navigate = useNavigate();
  const { criticalFindings, attackPathCount, affectedWorkloads } = data;
  const risksUrl = '/risks/findings';

  return (
    <div className="space-y-4">
      <DecisionSpine
        roleLabel="Platform owner"
        title={
          affectedWorkloads > 0
            ? `${plural(affectedWorkloads, 'workload')} need exposure review`
            : 'No scoped workload exposure requiring action'
        }
        situation="Start from service exposure, then inspect the attack path and decide whether remediation or escalation is needed."
        impact={`${plural(criticalFindings, 'critical finding')} and ${plural(attackPathCount, 'attack path')} in ${data.clusterName ?? 'the selected scope'}.`}
        owner={`${identityLabel}; remediation ownership follows the affected service team.`}
        actionLabel="Review observed threats"
        onAction={() => navigate(risksUrl)}
        secondaryAction={
          onOpenBrief ? (
            <Button type="button" variant="secondary" onClick={onOpenBrief}>
              Open executive brief
            </Button>
          ) : undefined
        }
        steps={[
          { label: 'Service at risk', value: plural(affectedWorkloads, 'workload'), detail: data.clusterName ?? 'Current scope' },
          { label: 'Attack path', value: plural(attackPathCount, 'path'), detail: 'Validate reachability' },
          { label: 'Impact', value: criticalFindings > 0 ? 'Critical exposure' : 'Scoped exposure', detail: 'Map to service owner' },
          { label: 'Remediation', value: 'Review threat evidence', detail: 'Choose fix or exception' },
          { label: 'Status', value: 'Track in case', detail: 'Escalate if blocked' },
        ]}
        metrics={[
          { label: 'Critical', value: criticalFindings, tone: criticalFindings > 0 ? 'critical' : 'neutral' },
          { label: 'Workloads', value: affectedWorkloads, tone: affectedWorkloads > 0 ? 'high' : 'neutral' },
        ]}
      />
      <div className="grid gap-3 lg:grid-cols-2">
        {sections.includes('threat_narrative') ? (
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
        {sections.includes('assigned_investigations') ? (
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
      </div>
    </div>
  );
};
