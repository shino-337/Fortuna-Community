import { useEffect, useState, useCallback } from 'react';
import {
  emitOperationalEvent,
  getOperationalEventVersion,
  getRecentOperationalEvents,
  subscribeOperationalEvents,
  type OperationalEvent,
  type OperationalEventType,
} from '../lib/operationalEvents';

/** Subscribes to operational events; bumps version for downstream useMemo deps. */
export function useOperationalEvents(): {
  version: number;
  recent: OperationalEvent[];
  emit: typeof emitOperationalEvent;
} {
  const [version, setVersion] = useState(getOperationalEventVersion);

  useEffect(() => {
    return subscribeOperationalEvents(() => {
      setVersion(getOperationalEventVersion());
    });
  }, []);

  const recent = getRecentOperationalEvents(12);

  const emit = useCallback(
    <T extends Record<string, unknown>>(type: OperationalEventType, payload: T, source?: string) =>
      emitOperationalEvent(type, payload, source),
    [],
  );

  return { version, recent, emit };
}
