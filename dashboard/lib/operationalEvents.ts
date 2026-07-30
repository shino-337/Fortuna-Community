/**
 * Operational event bus — event-driven cognition layer (client; server-authoritative later).
 */
export type OperationalEventType =
  | 'incident:phase_changed'
  | 'incident:case_selected'
  | 'telemetry:health_changed'
  | 'workspace:panel_changed'
  | 'workspace:graph_scope_changed'
  | 'workspace:decision_logged'
  | 'annotation:added'
  | 'command:role_assigned'
  | 'cognition:recompute_requested';

export interface OperationalEvent<T = Record<string, unknown>> {
  id: string;
  type: OperationalEventType;
  at: string;
  source: string;
  payload: T;
}

type Listener = (event: OperationalEvent) => void;

let version = 0;
const listeners = new Set<Listener>();
const recent: OperationalEvent[] = [];
const MAX_RECENT = 64;

function newEventId(): string {
  return `evt_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`;
}

/** Get the current operational event version (increments on each emit). */
export function getOperationalEventVersion(): number {
  return version;
}

/** Get recent operational events. */
export function getRecentOperationalEvents(limit?: number): OperationalEvent[];
/** @returns The most recent N operational events (up to the configured MAX_RECENT limit), useful for building activity feeds or audit trails. */
export function getRecentOperationalEvents(limit = 20): OperationalEvent[] {
  return recent.slice(0, limit);
}

/** Emit an operational event to all subscribers. */
export function emitOperationalEvent<T extends Record<string, unknown>>(
  type: OperationalEventType,
  payload: T,
  source?: string,
): OperationalEvent<T>;
/** @returns The emitted event object (for chaining or immediate use). Emits the event to all current subscribers and returns it. */
export function emitOperationalEvent<T extends Record<string, unknown>>(
  type: OperationalEventType,
  payload: T,
  source = 'client',
): OperationalEvent<T> {
  const event: OperationalEvent<T> = {
    id: newEventId(),
    type,
    at: new Date().toISOString(),
    source,
    payload,
  };
  version += 1;
  recent.unshift(event);
  if (recent.length > MAX_RECENT) recent.length = MAX_RECENT;
  for (const fn of listeners) {
    try {
      fn(event);
    } catch {
      /* listener fault isolation */
    }
  }
  return event;
}

export function subscribeOperationalEvents(listener: Listener): () => void;
/** @returns An unsubscribe callback. Call it to stop receiving events from this subscriber. */
export function subscribeOperationalEvents(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** Request a cognition recompute from the server. */
export function requestCognitionRecompute(reason: string, source?: string): void;
/** @param reason A human-readable explanation for why this recompute is being requested (e.g., "user changed graph scope"). Defaults to "client". */
export function requestCognitionRecompute(reason: string, source = 'client'): void {
  emitOperationalEvent('cognition:recompute_requested', { reason }, source);
}
