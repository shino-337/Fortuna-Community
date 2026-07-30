import React, { useState } from 'react';
import { FolderPlus, Check } from 'lucide-react';
import { Button } from './ui/Button';
import { api } from '../lib/api';
import type { InvestigationEntity } from '../store/investigationStore';
import { useInvestigationStore } from '../store/investigationStore';
import { useAuthStore } from '../store/authStore';
import { useClusterStore } from '../store/clusterStore';
import { useCan } from '../hooks/usePermUser';
import { P } from '../lib/permissions';

/** Button to pin an investigation entity (pod/node) to the active case — server-persisted. */
export const PinToInvestigationButton: React.FC<{
  entity: Omit<InvestigationEntity, 'id' | 'pinnedAt'>;
  size?: 'sm' | 'md';
  className?: string;
}> = ({ entity, size = 'sm', className }) => {
  const [flash, setFlash] = useState(false);
  const canWrite = useCan(P.investigationsWrite);
  const { user } = useAuthStore();
  const clusterId = useClusterStore((s) => s.selectedClusterId);
  const activeCaseId = useInvestigationStore((s) => s.activeCaseId);
  const setActiveCase = useInvestigationStore((s) => s.setActiveCase);

  if (!canWrite) return null;

  const pin = async () => {
    try {
      let caseId = activeCaseId;
      if (!caseId) {
        const created = await api.createInvestigationCase({
          title: `Case: ${entity.label.slice(0, 64)}`,
          owner: user?.username ?? user?.email ?? '',
          clusterId,
        });
        caseId = created.id;
        setActiveCase(caseId);
      }
      await api.pinInvestigationEntity(caseId, {
        type: entity.type,
        label: entity.label,
        href: entity.href,
        meta: entity.meta,
      });
      setFlash(true);
      window.setTimeout(() => setFlash(false), 1800);
    } catch {
      /* best-effort */
    }
  };

  return (
    <Button
      type="button"
      variant="secondary"
      size={size}
      className={className}
      onClick={() => void pin()}
      title="Add to investigation case (server-persisted)"
    >
      {flash ? <Check className="w-4 h-4 mr-1 text-emerald-400" /> : <FolderPlus className="w-4 h-4 mr-1" />}
      {flash ? 'Pinned' : 'Pin to case'}
    </Button>
  );
};
