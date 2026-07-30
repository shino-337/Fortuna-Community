/**
 * Operational memory — lightweight client-side patterns (UX hints, not authority).
 */
export interface OperationalMemoryPattern {
  id: string;
  kind:
    | 'recurring_blast_radius'
    | 'recurring_owner_gap'
    | 'remediation_effective'
    | 'remediation_stalled'
    | 'repeat_incident_cluster';
  label: string;
  occurrenceCount: number;
  lastSeenAt: string;
  hint: string;
}

export interface OperationalMemorySnapshot {
  patterns: OperationalMemoryPattern[];
  clusterIncidentCounts: Record<string, number>;
  ownerMissCount: Record<string, number>;
}

const STORAGE_KEY = 'fortuna-operational-memory-v1';

export function loadOperationalMemory(): OperationalMemorySnapshot {
  if (typeof window === 'undefined') {
    return { patterns: [], clusterIncidentCounts: {}, ownerMissCount: {} };
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { patterns: [], clusterIncidentCounts: {}, ownerMissCount: {} };
    return JSON.parse(raw) as OperationalMemorySnapshot;
  } catch {
    return { patterns: [], clusterIncidentCounts: {}, ownerMissCount: {} };
  }
}

export function saveOperationalMemory(snapshot: OperationalMemorySnapshot): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(snapshot));
  } catch {
    /* quota */
  }
}

export function recordIncidentObservation(input: {
  clusterId?: string | null;
  owner?: string | null;
  hadRemediation: boolean;
  resolved: boolean;
  entityCount: number;
}): OperationalMemorySnapshot {
  const mem = loadOperationalMemory();
  const now = new Date().toISOString();

  if (input.clusterId) {
    const k = String(input.clusterId);
    mem.clusterIncidentCounts[k] = (mem.clusterIncidentCounts[k] ?? 0) + 1;
    if (mem.clusterIncidentCounts[k] >= 3) {
      upsertPattern(mem, {
        id: `cluster_${k}`,
        kind: 'repeat_incident_cluster',
        label: `Recurring incidents on cluster ${k}`,
        occurrenceCount: mem.clusterIncidentCounts[k],
        lastSeenAt: now,
        hint: 'This cluster has repeated investigation activity — review baseline hardening.',
      });
    }
  }

  if (!input.owner?.trim()) {
    const k = input.clusterId ?? 'global';
    mem.ownerMissCount[k] = (mem.ownerMissCount[k] ?? 0) + 1;
    if (mem.ownerMissCount[k] >= 2) {
      upsertPattern(mem, {
        id: `owner_gap_${k}`,
        kind: 'recurring_owner_gap',
        label: 'Cases opened without assigned owner',
        occurrenceCount: mem.ownerMissCount[k],
        lastSeenAt: now,
        hint: 'Ownership gaps slow remediation — assign owner at triage.',
      });
    }
  }

  if (input.entityCount >= 5) {
    upsertPattern(mem, {
      id: 'blast_radius_wide',
      kind: 'recurring_blast_radius',
      label: 'Wide blast-radius pins',
      occurrenceCount: (mem.patterns.find((p) => p.id === 'blast_radius_wide')?.occurrenceCount ?? 0) + 1,
      lastSeenAt: now,
      hint: 'Large entity sets often indicate sprawling blast radius — prioritize crown jewels first.',
    });
  }

  if (input.hadRemediation && input.resolved) {
    upsertPattern(mem, {
      id: 'remediation_ok',
      kind: 'remediation_effective',
      label: 'Remediation completed with resolution',
      occurrenceCount: (mem.patterns.find((p) => p.id === 'remediation_ok')?.occurrenceCount ?? 0) + 1,
      lastSeenAt: now,
      hint: 'Recent remediations in this workspace reached resolved state.',
    });
  } else if (input.hadRemediation && !input.resolved) {
    upsertPattern(mem, {
      id: 'remediation_stall',
      kind: 'remediation_stalled',
      label: 'Remediation without resolution',
      occurrenceCount: (mem.patterns.find((p) => p.id === 'remediation_stall')?.occurrenceCount ?? 0) + 1,
      lastSeenAt: now,
      hint: 'Remediation steps exist but case is not resolved — check blockers and ownership.',
    });
  }

  saveOperationalMemory(mem);
  return mem;
}

function upsertPattern(mem: OperationalMemorySnapshot, pattern: OperationalMemoryPattern): void {
  const idx = mem.patterns.findIndex((p) => p.id === pattern.id);
  if (idx >= 0) mem.patterns[idx] = pattern;
  else mem.patterns.push(pattern);
  mem.patterns = mem.patterns
    .sort((a, b) => b.occurrenceCount - a.occurrenceCount)
    .slice(0, 12);
}
