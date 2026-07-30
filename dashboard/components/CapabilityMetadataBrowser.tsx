import React from 'react';
import { useNavigate } from 'react-router-dom';
import { CapabilityMetadata, SecurityRule } from '../types';
import { Info, ChevronRight } from 'lucide-react';
import { getSeverityBadgeClass } from '../lib/severity';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../lib/tableChrome';

const tooltipLabelClass = 'inline-flex items-center gap-1 underline decoration-dotted underline-offset-2 cursor-help';

function ruleCountForCapability(rules: SecurityRule[], capabilityId: string): number {
  let n = 0;
  for (const r of rules) {
    if ((r.relatedCapabilities || []).includes(capabilityId)) n += 1;
  }
  return n;
}

export interface CapabilityMetadataBrowserProps {
  metadata: CapabilityMetadata[];
  rules: SecurityRule[];
  loading?: boolean;
}

/** Table list aligned with Resources / Rules; details on `/capabilities/:id`. */
export const CapabilityMetadataBrowser: React.FC<CapabilityMetadataBrowserProps> = ({ metadata, rules, loading }) => {
  const navigate = useNavigate();

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="w-10 h-10 border-4 border-brand border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (metadata.length === 0) {
    return (
      <div className="text-center py-12 text-muted">
        <Info size={24} className="mx-auto mb-2 opacity-50" />
        <p>No capabilities match the current filters.</p>
      </div>
    );
  }

  return (
    <div className="ui-table-scroll">
      <table className={UI_TABLE}>
        <thead className={UI_THEAD_STICKY}>
          <tr>
            <th className={UI_TH}>Capability</th>
            <th className={UI_TH}>Domain</th>
            <th className={UI_TH}>Severity</th>
            <th className={UI_TH}>MITRE</th>
            <th className={UI_TH}>
              <span className={tooltipLabelClass} title="Rules whose findings often reference this capability in evidence">
                Linked rules <Info className="w-3 h-3" />
              </span>
            </th>
            <th className={`${UI_TH} text-right`}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {metadata.map((meta) => {
            const rc = ruleCountForCapability(rules, meta.capabilityId);
            return (
              <tr key={meta.capabilityId} className={UI_TR}>
                <td className={UI_TD}>
                  <div className="font-medium text-text">{meta.name || meta.capabilityId}</div>
                  <div className="text-caption text-muted font-mono">{meta.capabilityId}</div>
                  {(meta.summary || meta.description) && (
                    <div className="text-caption text-muted mt-1 max-w-xl line-clamp-2">{meta.summary || meta.description}</div>
                  )}
                </td>
                <td className={`${UI_TD} text-muted`}>{meta.domain}</td>
                <td className={UI_TD}>
                  <span className={`px-2 py-0.5 rounded text-caption font-medium border ${getSeverityBadgeClass(meta.severityBase?.toLowerCase())}`}>
                    {meta.severityBase}
                  </span>
                </td>
                <td className={`${UI_TD} text-caption text-muted`}>
                  {meta.mitreTactic && <div>{meta.mitreTactic}</div>}
                  {meta.mitreTechnique && <div className="font-mono text-amber-400/90">{meta.mitreTechnique}</div>}
                  {!meta.mitreTactic && !meta.mitreTechnique && <span className="text-muted-2">—</span>}
                </td>
                <td className={`${UI_TD} text-muted text-caption tabular-nums`}>{rc}</td>
                <td className={`${UI_TD} text-right`}>
                  <button
                    type="button"
                    onClick={() => navigate(`/capabilities/${encodeURIComponent(meta.capabilityId)}`)}
                    className="inline-flex items-center gap-1 text-brand hover:text-brand text-caption font-medium"
                  >
                    Details <ChevronRight className="w-3.5 h-3.5" />
                  </button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
};
