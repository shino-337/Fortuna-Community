import React from 'react';
import { caseStatusLabel } from '../../lib/caseLifecycle';
import type { InvestigationStatus } from '../../store/investigationStore';

const TONE: Record<InvestigationStatus, string> = {
  OPEN: 'border-border text-text',
  TRIAGED: 'border-border text-text',
  ACTIVE: 'border-brand/50 text-brand',
  CONTAINED: 'border-amber-500/50 text-amber-300',
  REMEDIATING: 'border-amber-500/50 text-amber-300',
  RESOLVED: 'border-emerald-500/50 text-emerald-300',
  ARCHIVED: 'border-border text-muted',
};

export const CaseStatusBadge: React.FC<{ status: InvestigationStatus }> = ({ status }) => (
  <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-meta font-semibold ${TONE[status] ?? TONE.OPEN}`}>
    {caseStatusLabel(status)}
  </span>
);
