import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { CommandRole, IncidentCommandAssignment } from '../lib/incidentCommand';
import { mergeCommandAssignment } from '../lib/incidentCommand';
import { emitOperationalEvent } from '../lib/operationalEvents';

interface IncidentCommandState {
  byCaseId: Record<string, IncidentCommandAssignment>;
  setRole: (caseId: string, role: CommandRole, username: string) => void;
  setDelegates: (caseId: string, delegates: string[]) => void;
  getAssignment: (caseId: string) => IncidentCommandAssignment | undefined;
}

export const useIncidentCommandStore = create<IncidentCommandState>()(
  persist(
    (set, get) => ({
      byCaseId: {},
      /** Get the command assignment for a case. */
      getAssignment: (caseId) => get().byCaseId[caseId],
      /** Set a role for the incident commander. */
      setRole: (caseId, role, username) => {
        const cur = get().byCaseId[caseId] ?? {
          caseId,
          roles: {},
          delegates: [],
          updatedAt: new Date().toISOString(),
        };
        const next = mergeCommandAssignment(cur, { [role]: username });
        set((s) => ({ byCaseId: { ...s.byCaseId, [caseId]: next } }));
        emitOperationalEvent('command:role_assigned', { caseId, role, username });
      },
      /** Set delegates for a command. */
      setDelegates: (caseId, delegates) => {
        const cur = get().byCaseId[caseId] ?? {
          caseId,
          roles: {},
          delegates: [],
          updatedAt: new Date().toISOString(),
        };
        const next = mergeCommandAssignment(cur, { delegates });
        set((s) => ({ byCaseId: { ...s.byCaseId, [caseId]: next } }));
      },
    }),
    { name: 'fortuna-incident-command-v1' },
  ),
);
