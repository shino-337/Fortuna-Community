import React from 'react';
import { Shield } from 'lucide-react';
import {
  buildIncidentCommandStructure,
  deriveCommandAssignment,
  roleLabel,
  type CommandRole,
} from '../lib/incidentCommand';
import { useIncidentCommandStore } from '../store/incidentCommandStore';
import type { InvestigationCase } from '../store/investigationStore';

const EDITABLE_ROLES: CommandRole[] = [
  'commander',
  'containment_lead',
  'remediation_lead',
  'communications_owner',
];

export const IncidentCommandPanel: React.FC<{
  activeCase: InvestigationCase;
  canWrite: boolean;
}> = ({ activeCase, canWrite }) => {
  const stored = useIncidentCommandStore((s) => s.getAssignment(activeCase.id));
  const setRole = useIncidentCommandStore((s) => s.setRole);
  const base = stored ?? deriveCommandAssignment(activeCase);
  const structure = buildIncidentCommandStructure(base);

  return (
    <section className="rounded-lg border border-border bg-base/30 p-3 space-y-3" aria-label="Incident command">
      <h3 className="text-caption font-semibold uppercase tracking-wider text-muted flex items-center gap-2">
        <Shield className="w-4 h-4" />
        Incident command ({structure.coveragePercent}% coverage)
      </h3>
      <p className="text-meta text-muted">{structure.summary}</p>
      <div className="grid gap-2 sm:grid-cols-2">
        {EDITABLE_ROLES.map((role) => (
          <label key={role} className="block text-meta">
            <span className="text-muted uppercase tracking-wide">{roleLabel(role)}</span>
            <input
              type="text"
              value={base.roles[role] ?? ''}
              disabled={!canWrite}
              placeholder="Assign…"
              onChange={(e) => setRole(activeCase.id, role, e.target.value)}
              className="mt-1 w-full rounded-md border border-border bg-base px-2 py-1.5 text-caption disabled:opacity-60"
            />
          </label>
        ))}
      </div>
    </section>
  );
};
