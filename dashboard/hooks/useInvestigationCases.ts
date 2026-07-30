import { useCallback, useEffect, useState } from 'react';
import { api, type InvestigationCaseApi } from '../lib/api';
import {
  useInvestigationStore,
  type InvestigationCase,
  type RemediationActionStatus,
} from '../store/investigationStore';
import { useClusterStore } from '../store/clusterStore';
import { useAuthStore } from '../store/authStore';
import { useCan } from './usePermUser';
import { P } from '../lib/permissions';

function normalizeStatus(raw: string): InvestigationCase['status'] {
  const s = raw.toUpperCase();
  const allowed: InvestigationCase['status'][] = [
    'OPEN',
    'TRIAGED',
    'ACTIVE',
    'CONTAINED',
    'REMEDIATING',
    'RESOLVED',
    'ARCHIVED',
  ];
  if (allowed.includes(s as InvestigationCase['status'])) {
    return s as InvestigationCase['status'];
  }
  if (s === 'TRIAGING') return 'TRIAGED';
  if (s === 'CLOSED') return 'RESOLVED';
  return 'OPEN';
}


function normalizeRemediationStatus(raw: string): RemediationActionStatus {
  const s = raw.toLowerCase();
  if (s === 'pending' || s === 'in_progress' || s === 'done' || s === 'blocked') return s;
  if (s === 'complete' || s === 'completed' || s === 'resolved') return 'done';
  if (s === 'active' || s === 'running') return 'in_progress';
  return 'pending';
}

function mapCase(row: InvestigationCaseApi): InvestigationCase {
  return {
    id: row.id,
    title: row.title,
    status: normalizeStatus(row.status),
    owner: row.owner,
    clusterId: row.clusterId ?? null,
    notes: row.notes ?? [],
    entities: (row.entities ?? []).map((e) => ({
      id: e.id,
      type: e.type as InvestigationCase['entities'][0]['type'],
      label: e.label,
      href: e.href,
      meta: e.meta,
      pinnedAt: e.pinnedAt,
      snapshot: e.snapshot,
    })),
    remediationActions: (row.remediationActions ?? []).map((a) => ({
      ...a,
      status: normalizeRemediationStatus(a.status),
    })),
    collaboration: row.collaboration ?? { assignees: [], watchers: [], handoffNotes: [] },
    createdAt: row.createdAt,
    updatedAt: row.updatedAt,
    slaDueAt: row.slaDueAt ?? undefined,
    archivedAt: row.archivedAt ?? undefined,
    retentionUntil: row.retentionUntil ?? undefined,
  };
}

/**
 * Hook that manages investigation cases with CRUD operations and permissions.
 */
export function useInvestigationCases() {
  const canRead = useCan(P.investigationsRead);
  const canWrite = useCan(P.investigationsWrite);
  const canDelete = useCan(P.investigationsDelete);
  const clusterId = useClusterStore((s) => s.selectedClusterId);
  const { user } = useAuthStore();
  const activeCaseId = useInvestigationStore((s) => s.activeCaseId);
  const setActiveCase = useInvestigationStore((s) => s.setActiveCase);

  const [cases, setCases] = useState<InvestigationCase[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [source, setSource] = useState<'server' | 'unavailable'>('unavailable');

  const refresh = useCallback(async () => {
    if (!canRead) {
      setCases([]);
      setSource('unavailable');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const { items } = await api.listInvestigationCases();
      setCases(items.map(mapCase));
      setSource('server');
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
      setCases([]);
    } finally {
      setLoading(false);
    }
  }, [canRead]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const createCase = useCallback(
    async (partial?: { title?: string; owner?: string }) => {
      if (!canWrite) throw new Error('Missing investigations.write permission');
      const author = user?.username ?? user?.email ?? '';
      const created = await api.createInvestigationCase({
        title: partial?.title,
        owner: partial?.owner ?? author,
        clusterId,
      });
      const mapped = mapCase(created);
      setActiveCase(mapped.id);
      await refresh();
      return mapped.id;
    },
    [canWrite, user, clusterId, setActiveCase, refresh],
  );

  const updateCase = useCallback(
    async (id: string, patch: Partial<InvestigationCase>) => {
      if (!canWrite) throw new Error('Missing investigations.write permission');
      await api.patchInvestigationCase(id, {
        title: patch.title,
        status: patch.status,
        owner: patch.owner,
        clusterId: patch.clusterId,
        slaDueAt: patch.slaDueAt ?? null,
        entities: patch.entities,
        notes: patch.notes,
        remediationActions: patch.remediationActions,
        collaboration: patch.collaboration,
      });
      await refresh();
    },
    [canWrite, refresh],
  );

  const deleteCase = useCallback(
    async (id: string) => {
      if (!canDelete) throw new Error('Missing investigations.delete permission');
      await api.deleteInvestigationCase(id);
      if (activeCaseId === id) setActiveCase(null);
      await refresh();
    },
    [canDelete, activeCaseId, setActiveCase, refresh],
  );

  const addEntity = useCallback(
    async (caseId: string, entity: Omit<InvestigationCase['entities'][0], 'pinnedAt' | 'id'> & { id?: string }) => {
      if (!canWrite) return false;
      const res = await api.pinInvestigationEntity(caseId, {
        type: entity.type,
        label: entity.label,
        href: entity.href,
        meta: entity.meta,
      });
      await refresh();
      return res.pinned;
    },
    [canWrite, refresh],
  );

  return {
    cases,
    loading,
    error,
    source,
    canRead,
    canWrite,
    canDelete,
    activeCaseId,
    setActiveCase,
    refresh,
    createCase,
    updateCase,
    deleteCase,
    addEntity,
  };
}
