import React from 'react';
import { ArrowRightLeft } from 'lucide-react';
import { useInvestigationCases } from '../hooks/useInvestigationCases';
import { useInvestigationStore } from '../store/investigationStore';

/** Banner showing the latest handoff note for the active investigation case. */
export const HandoffBanner: React.FC = () => {
  const { cases } = useInvestigationCases();
  const activeCaseId = useInvestigationStore((s) => s.activeCaseId);
  const active = cases.find((c) => c.id === activeCaseId) ?? cases[0];
  const notes = active?.collaboration?.handoffNotes?.filter((note) => note.body.trim()) ?? [];

  if (!active || notes.length === 0) return null;

  return (
    <div className="mb-4 rounded-lg border border-brand/30 bg-brand/5 px-3 py-2 text-caption">
      <p className="font-semibold text-brand flex items-center gap-1.5 mb-1">
        <ArrowRightLeft className="w-4 h-4" aria-hidden />
        Handoff notes
      </p>
      <p className="text-muted whitespace-pre-wrap">{notes[notes.length - 1]?.body}</p>
    </div>
  );
};
