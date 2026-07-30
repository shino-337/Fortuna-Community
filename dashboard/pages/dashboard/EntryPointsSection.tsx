import React from 'react';
import { Eye, PanelRight, Target } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { getSeverityBadgeClass } from '../../lib/severity';
import { UI_TABLE, UI_TH, UI_TR, UI_TD_COMPACT } from '../../lib/tableChrome';
import type { EntryScenario } from './types';
import { riskLabelTitle } from './utils';

export interface EntryPointsSectionProps {
  scenarios: EntryScenario[];
  onOpenPanel: (scenario: EntryScenario) => void;
  onOpenPod: (uid: string) => void;
  onOpenAttackPaths: (uid: string) => void;
}

export const EntryPointsSection: React.FC<EntryPointsSectionProps> = ({
  scenarios,
  onOpenPanel,
  onOpenPod,
  onOpenAttackPaths,
}) => {
  if (scenarios.length === 0) {
    return <p className="text-caption text-muted-2">No entry points match the current filters.</p>;
  }

  const shown = scenarios.slice(0, 10);

  return (
    <>
      <div className="md:hidden space-y-2">
        {shown.map((scenario) => (
          <article
            key={scenario.key}
            className="rounded-lg border border-border/70 bg-surface/30 p-3 space-y-2"
          >
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="min-w-0">
                <p className="font-semibold text-body text-text truncate">{scenario.pod.name}</p>
                <p className="font-mono text-meta text-muted-2 truncate">{scenario.pod.namespace}</p>
              </div>
              <span
                className={`shrink-0 rounded border px-1.5 py-0.5 text-micro font-bold ${getSeverityBadgeClass(scenario.riskLevel)}`}
              >
                {riskLabelTitle(scenario.riskLevel)}
              </span>
            </div>
            {scenario.runtimeConfirmed ? (
              <span className="inline-flex items-center gap-1 rounded border border-emerald-500/30 bg-emerald-500/10 px-1.5 py-0.5 text-micro text-emerald-300">
                <Eye className="h-3 w-3" aria-hidden /> Runtime confirmed
              </span>
            ) : null}
            <div className="flex flex-wrap gap-1.5 pt-1">
              <Button variant="secondary" size="sm" className="!text-caption" onClick={() => onOpenPanel(scenario)}>
                Path detail
              </Button>
              <Button variant="ghost" size="sm" className="!text-caption" onClick={() => onOpenAttackPaths(scenario.pod.uid)}>
                Attack paths
              </Button>
              <Button variant="ghost" size="sm" className="!text-caption" onClick={() => onOpenPod(scenario.pod.uid)}>
                Pod
              </Button>
            </div>
          </article>
        ))}
      </div>

      <div className="hidden md:block overflow-x-auto -mx-0.5 rounded-lg border border-border/50">
        <table className={`${UI_TABLE} min-w-[min(100%,36rem)]`}>
          <thead>
            <tr>
              <th className={UI_TH}>Entry point</th>
              <th className={`${UI_TH} w-[1%] whitespace-nowrap text-right`}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {shown.map((scenario) => (
              <tr key={scenario.key} className={UI_TR}>
                <td className={UI_TD_COMPACT}>
                  <div className="flex min-w-0 flex-col gap-1.5 sm:flex-row sm:flex-wrap sm:items-center sm:gap-2">
                    <span className="truncate font-semibold text-body text-text">{scenario.pod.name}</span>
                    <span className="truncate font-mono text-meta text-muted-2">{scenario.pod.namespace}</span>
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span
                        className={`shrink-0 rounded border px-1.5 py-0.5 text-micro font-bold ${getSeverityBadgeClass(scenario.riskLevel)}`}
                      >
                        Risk level: {riskLabelTitle(scenario.riskLevel)}
                      </span>
                      {scenario.runtimeConfirmed ? (
                        <span className="inline-flex items-center gap-1 rounded border border-emerald-500/30 bg-emerald-500/10 px-1.5 py-0.5 text-micro text-emerald-300">
                          <Eye className="h-3 w-3" aria-hidden /> Runtime confirmed
                        </span>
                      ) : null}
                    </div>
                  </div>
                </td>
                <td className={`${UI_TD_COMPACT} whitespace-nowrap text-right`}>
                  <div className="flex flex-wrap items-center justify-end gap-1">
                    <Button variant="secondary" size="sm" className="h-8 !text-caption" onClick={() => onOpenPanel(scenario)}>
                      Path detail
                    </Button>
                    <button
                      type="button"
                      aria-label={`Open path panel for ${scenario.pod.name}`}
                      onClick={() => onOpenPanel(scenario)}
                      className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-surface/50 text-muted hover:border-brand/40 hover:text-brand"
                    >
                      <PanelRight className="h-4 w-4" />
                    </button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-8 !text-caption"
                      onClick={() => onOpenAttackPaths(scenario.pod.uid)}
                    >
                      <Target className="h-3.5 w-3.5 mr-1 inline" aria-hidden />
                      Paths
                    </Button>
                    <Button variant="ghost" size="sm" className="h-8 !text-caption" onClick={() => onOpenPod(scenario.pod.uid)}>
                      Pod
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
};
