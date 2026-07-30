import React, { useState } from 'react';
import {
  EMPTY_WORKSPACE,
  useInvestigationWorkspaceStore,
} from '../../store/investigationWorkspaceStore';
import { Button } from '../ui/Button';

export const DecisionLogPanel: React.FC<{
  caseId: string;
  author: string;
  canWrite: boolean;
}> = ({ caseId, author, canWrite }) => {
  const workspace = useInvestigationWorkspaceStore((s) => s.byCaseId[caseId] ?? EMPTY_WORKSPACE);
  const appendDecision = useInvestigationWorkspaceStore((s) => s.appendDecision);
  const [decision, setDecision] = useState('');
  const [rationale, setRationale] = useState('');

  return (
    <div className="space-y-3">
      {canWrite ? (
        <div className="rounded-lg border border-border bg-base/30 p-3 space-y-2">
          <label className="text-meta text-muted uppercase tracking-wide">Record decision</label>
          <input
            type="text"
            value={decision}
            onChange={(e) => setDecision(e.target.value)}
            placeholder="e.g. Contain pod X — isolate namespace"
            className="w-full rounded-md border border-border bg-base px-3 py-2 text-caption"
          />
          <textarea
            value={rationale}
            onChange={(e) => setRationale(e.target.value)}
            placeholder="Why — evidence, blast radius, runtime confirmation…"
            rows={2}
            className="w-full rounded-md border border-border bg-base px-3 py-2 text-caption"
          />
          <Button
            type="button"
            size="sm"
            disabled={!decision.trim()}
            onClick={() => {
              appendDecision(caseId, {
                author,
                decision: decision.trim(),
                rationale: rationale.trim() || 'No rationale recorded.',
                confidence: 0.8,
              });
              setDecision('');
              setRationale('');
            }}
          >
            Append to decision log
          </Button>
        </div>
      ) : null}
      {workspace.decisionLog.length === 0 ? (
        <p className="text-caption text-muted">No decisions logged yet — record containment, escalation, or scope calls.</p>
      ) : (
        <ul className="space-y-2 max-h-[min(50vh,24rem)] overflow-y-auto">
          {workspace.decisionLog.map((e) => (
            <li key={e.id} className="rounded-md border border-border bg-base/40 px-3 py-2 text-caption">
              <div className="flex justify-between gap-2 text-meta text-muted">
                <span>{e.author}</span>
                <span>{new Date(e.at).toLocaleString()}</span>
              </div>
              <p className="font-semibold text-text mt-1">{e.decision}</p>
              <p className="text-muted mt-0.5">{e.rationale}</p>
              <p className="text-meta text-muted-2 mt-1">Confidence: {Math.round(e.confidence * 100)}%</p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};
