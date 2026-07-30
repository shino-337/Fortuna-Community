import { useEffect, useRef, useCallback } from 'react';

/**
 * Returns a stable function that creates an AbortSignal tied to the
 * component lifecycle. Each call cancels the previous signal.
 *
 * Usage:
 *   const getSignal = useAbortSignal();
 *   const signal = getSignal();  // previous aborted
 *   const res = await fetch(url, { signal });
 *
 * Automatically aborts on unmount.
 */
export function useAbortSignal(): () => AbortSignal {
  const controllerRef = useRef<AbortController | null>(null);

  // Abort on unmount
  useEffect(() => {
    return () => {
      controllerRef.current?.abort();
    };
  }, []);

  return useCallback(() => {
    // Cancel any previous in-flight request
    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;
    return controller.signal;
  }, []);
}

/**
 * Guard for checking if an error is an AbortError (should be silently ignored).
 */
export function isAbortError(err: unknown): boolean {
  if (err instanceof DOMException && err.name === 'AbortError') return true;
  if (err instanceof Error && err.name === 'AbortError') return true;
  return false;
}
