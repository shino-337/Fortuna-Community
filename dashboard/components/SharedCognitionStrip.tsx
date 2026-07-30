import React, { useEffect } from 'react';
import { Radio, MessageSquare } from 'lucide-react';
import { EMPTY_ANNOTATIONS, EMPTY_PRESENCE, useSharedCognitionStore } from '../store/sharedCognitionStore';
import { useInvestigationStore } from '../store/investigationStore';
import { useInvestigationWorkspaceStore } from '../store/investigationWorkspaceStore';
import { usePermUser } from '../hooks/usePermUser';

/** Strip showing shared cognition — cross-tab presence and live annotations. */
export const SharedCognitionStrip: React.FC = () => {
  const user = usePermUser();
  const activeCaseId = useInvestigationStore((s) => s.activeCaseId);
  const panel = useInvestigationWorkspaceStore((s) =>
    activeCaseId ? s.getWorkspace(activeCaseId).activePanel : 'overview',
  );
  const initChannel = useSharedCognitionStore((s) => s.initChannel);
  const broadcastPresence = useSharedCognitionStore((s) => s.broadcastPresence);
  const presence = useSharedCognitionStore((s) =>
    activeCaseId ? s.presenceByCase[activeCaseId] ?? EMPTY_PRESENCE : EMPTY_PRESENCE,
  );
  const annotations = useSharedCognitionStore((s) =>
    activeCaseId ? s.annotationsByCase[activeCaseId] ?? EMPTY_ANNOTATIONS : EMPTY_ANNOTATIONS,
  );

  useEffect(() => initChannel(), [initChannel]);

  useEffect(() => {
    if (!activeCaseId || !user?.username) return;
    const id = window.setInterval(() => {
      broadcastPresence({
        caseId: activeCaseId,
        userId: user.username,
        displayName: user.username,
        panel,
      });
    }, 8000);
    broadcastPresence({
      caseId: activeCaseId,
      userId: user.username,
      displayName: user.username,
      panel,
    });
    return () => window.clearInterval(id);
  }, [activeCaseId, user?.username, panel, broadcastPresence]);

  if (!activeCaseId) return null;
  const others = presence.filter((p) => p.userId !== user?.username);

  return (
    <div className="rounded-lg border border-border/80 bg-surface/40 px-3 py-2 text-caption mb-3">
      <p className="font-semibold text-text flex items-center gap-2">
        <Radio className="w-4 h-4 text-brand" />
        Shared cognition
      </p>
      {others.length > 0 ? (
        <p className="text-muted mt-1">
          Active: {others.map((p) => `${p.displayName} (${p.panel})`).join(' · ')}
        </p>
      ) : (
        <p className="text-muted mt-1">No other responders in this case (cross-tab presence).</p>
      )}
      {annotations.length > 0 ? (
        <ul className="mt-2 space-y-1 border-t border-border/60 pt-2">
          {annotations.slice(0, 3).map((a) => (
            <li key={a.id} className="flex items-start gap-1.5 text-meta">
              <MessageSquare className="w-3 h-3 shrink-0 mt-0.5 text-muted" />
              <span>
                <span className="text-muted">{a.author}:</span> {a.body}
              </span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
};
