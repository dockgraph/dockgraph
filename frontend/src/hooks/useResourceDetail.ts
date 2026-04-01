import { useCallback, useEffect, useRef, useSyncExternalStore } from 'react';

interface DetailResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
}

const EMPTY: DetailResult<never> = { data: null, loading: false, error: null };

/**
 * Generic fetch-on-change hook for Docker resource inspection endpoints.
 * Cancels in-flight requests when the URL changes or on unmount.
 *
 * Uses useSyncExternalStore to manage the reset-on-null path synchronously
 * without triggering cascading renders or accessing refs during render.
 */
export function useResourceDetail<T>(url: string | null): DetailResult<T> {
  const stateRef = useRef<DetailResult<T>>(EMPTY);
  const abortRef = useRef<AbortController | null>(null);
  const subscribersRef = useRef(new Set<() => void>());

  const subscribe = useCallback((cb: () => void) => {
    subscribersRef.current.add(cb);
    return () => { subscribersRef.current.delete(cb); };
  }, []);

  const notify = useCallback(() => {
    for (const cb of subscribersRef.current) cb();
  }, []);

  const getSnapshot = useCallback(() => stateRef.current, []);

  const state = useSyncExternalStore(subscribe, getSnapshot);

  useEffect(() => {
    abortRef.current?.abort();
    abortRef.current = null;

    if (!url) {
      if (stateRef.current !== EMPTY) {
        stateRef.current = EMPTY;
        notify();
      }
      return;
    }

    stateRef.current = { ...stateRef.current, loading: true, error: null };
    notify();

    const controller = new AbortController();
    abortRef.current = controller;

    fetch(url, { signal: controller.signal })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: T) => {
        if (!controller.signal.aborted) {
          stateRef.current = { data, loading: false, error: null };
          notify();
        }
      })
      .catch((err) => {
        if (!controller.signal.aborted) {
          stateRef.current = { data: null, loading: false, error: err.message };
          notify();
        }
      });

    return () => controller.abort();
  }, [url, notify]);

  return state;
}
