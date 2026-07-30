import React, { useState } from 'react';
import { MessageSquarePlus } from 'lucide-react';
import { EMPTY_ANNOTATIONS, useSharedCognitionStore } from '../store/sharedCognitionStore';
import { Button } from './ui/Button';

/** Panel for live team annotations — broadcast notes visible to all investigators on a case. */
export const LiveAnnotationsPanel: React.FC<{
  caseId: string;
  author: string;
  canWrite: boolean;
}> = ({ caseId, author, canWrite }) => {
  const addAnnotation = useSharedCognitionStore((s) => s.addAnnotation);
  const annotations = useSharedCognitionStore((s) => s.annotationsByCase[caseId] ?? EMPTY_ANNOTATIONS);
  const [body, setBody] = useState('');

  return (
    <div className="space-y-2">
      <h4 className="text-meta text-muted uppercase tracking-wide flex items-center gap-1.5">
        <MessageSquarePlus className="w-3.5 h-3.5" />
        Live annotations (team)
      </h4>
      {canWrite ? (
        <div className="flex gap-2">
          <input
            type="text"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="Broadcast to other investigators on this case…"
            className="flex-1 rounded-md border border-border bg-base px-2 py-1.5 text-caption"
          />
          <Button
            type="button"
            size="sm"
            disabled={!body.trim()}
            onClick={() => {
              addAnnotation({ caseId, author, body: body.trim() });
              setBody('');
            }}
          >
            Post
          </Button>
        </div>
      ) : null}
      <ul className="space-y-1 max-h-32 overflow-y-auto">
        {annotations.length === 0 ? (
          <li className="text-meta text-muted">No live annotations yet.</li>
        ) : (
          annotations.map((a) => (
            <li key={a.id} className="text-caption rounded border border-border/60 px-2 py-1 bg-base/20">
              <span className="text-meta text-muted">{a.author}</span> · {a.body}
            </li>
          ))
        )}
      </ul>
    </div>
  );
};
