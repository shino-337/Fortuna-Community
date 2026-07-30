/**
 * Team-shared cognition — cross-tab broadcast for workspace, graph scope, presence, annotations.
 * Client-side until server-authoritative sync exists.
 */
import { create } from 'zustand';
import { emitOperationalEvent } from '../lib/operationalEvents';
import type { WorkspacePanel } from './investigationWorkspaceStore';

export interface SharedPresence {
  caseId: string;
  userId: string;
  displayName: string;
  panel: WorkspacePanel;
  lastSeenAt: string;
}

export const EMPTY_PRESENCE: SharedPresence[] = [];

export interface LiveAnnotation {
  id: string;
  caseId: string;
  author: string;
  body: string;
  at: string;
  anchorEntityId?: string;
}

export const EMPTY_ANNOTATIONS: LiveAnnotation[] = [];

type BroadcastMessage =
  | { kind: 'presence'; payload: SharedPresence }
  | { kind: 'annotation'; payload: LiveAnnotation }
  | { kind: 'graph_focus'; payload: { caseId: string; entityId: string | null; by: string } };

const CHANNEL_NAME = 'fortuna-shared-cognition-v1';

let channel: BroadcastChannel | null = null;

function getChannel(): BroadcastChannel | null {
  if (typeof window === 'undefined' || typeof BroadcastChannel === 'undefined') return null;
  if (!channel) channel = new BroadcastChannel(CHANNEL_NAME);
  return channel;
}

interface SharedCognitionState {
  presenceByCase: Record<string, SharedPresence[]>;
  annotationsByCase: Record<string, LiveAnnotation[]>;
  broadcastPresence: (p: Omit<SharedPresence, 'lastSeenAt'>) => void;
  addAnnotation: (a: Omit<LiveAnnotation, 'id' | 'at'>) => void;
  broadcastGraphFocus: (caseId: string, entityId: string | null, by: string) => void;
  initChannel: () => () => void;
}

function newAnnId(): string {
  return `ann_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`;
}

export const useSharedCognitionStore = create<SharedCognitionState>((set, get) => ({
  presenceByCase: {},
  annotationsByCase: {},

  /** Broadcast user presence to other tabs. */
  broadcastPresence: (p) => {
    const payload: SharedPresence = { ...p, lastSeenAt: new Date().toISOString() };
    set((s) => {
      const list = [...(s.presenceByCase[p.caseId] ?? [])].filter((x) => x.userId !== p.userId);
      list.push(payload);
      const trimmed = list.filter((x) => Date.now() - new Date(x.lastSeenAt).getTime() < 120_000);
      return { presenceByCase: { ...s.presenceByCase, [p.caseId]: trimmed } };
    });
    getChannel()?.postMessage({ kind: 'presence', payload } satisfies BroadcastMessage);
    emitOperationalEvent('workspace:panel_changed', { caseId: p.caseId, panel: p.panel, userId: p.userId });
  },

  /** Add a live annotation to the current user's session. */
  addAnnotation: (a) => {
    const payload: LiveAnnotation = {
      ...a,
      id: newAnnId(),
      at: new Date().toISOString(),
    };
    set((s) => ({
      annotationsByCase: {
        ...s.annotationsByCase,
        [a.caseId]: [payload, ...(s.annotationsByCase[a.caseId] ?? [])].slice(0, 40),
      },
    }));
    getChannel()?.postMessage({ kind: 'annotation', payload } satisfies BroadcastMessage);
    emitOperationalEvent('annotation:added', { caseId: a.caseId, author: a.author });
  },

  /** Broadcast graph focus to other tabs. */
  broadcastGraphFocus: (caseId, entityId, by) => {
    getChannel()?.postMessage({
      kind: 'graph_focus',
      payload: { caseId, entityId, by },
    } satisfies BroadcastMessage);
    emitOperationalEvent('workspace:graph_scope_changed', { caseId, entityId, by });
  },

  /** Initialize the broadcast channel and return cleanup function. */
  initChannel: () => {
    const ch = getChannel();
    if (!ch) return () => undefined;
    const onMessage = (ev: MessageEvent<BroadcastMessage>) => {
      const msg = ev.data;
      if (!msg?.kind) return;
      if (msg.kind === 'presence') {
        set((s) => {
          const list = [...(s.presenceByCase[msg.payload.caseId] ?? [])].filter(
            (x) => x.userId !== msg.payload.userId,
          );
          list.push(msg.payload);
          return {
            presenceByCase: {
              ...s.presenceByCase,
              [msg.payload.caseId]: list.slice(-12),
            },
          };
        });
      } else if (msg.kind === 'annotation') {
        set((s) => ({
          annotationsByCase: {
            ...s.annotationsByCase,
            [msg.payload.caseId]: [msg.payload, ...(s.annotationsByCase[msg.payload.caseId] ?? [])].slice(0, 40),
          },
        }));
      }
    };
    ch.addEventListener('message', onMessage);
    return () => ch.removeEventListener('message', onMessage);
  },
}));
