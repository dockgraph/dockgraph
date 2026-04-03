import { useCallback, useEffect, useRef } from "react";
import { useSyncExternalStore } from "react";

interface FetchState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
}

const EMPTY: FetchState<never> = { data: null, loading: false, error: null };

export function usePollingFetch<T>(url: string | null, intervalMs: number): FetchState<T> {
  const stateRef = useRef<FetchState<T>>(EMPTY as FetchState<T>);
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
    if (!url) {
      if (stateRef.current !== (EMPTY as FetchState<T>)) {
        stateRef.current = EMPTY as FetchState<T>;
        notify();
      }
      return;
    }

    const controller = new AbortController();

    const doFetch = () => {
      fetch(url, { signal: controller.signal })
        .then(res => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
          return res.json();
        })
        .then((data: T) => {
          if (!controller.signal.aborted) {
            stateRef.current = { data, loading: false, error: null };
            notify();
          }
        })
        .catch(err => {
          if (!controller.signal.aborted) {
            stateRef.current = { data: stateRef.current.data, loading: false, error: err.message };
            notify();
          }
        });
    };

    stateRef.current = { ...stateRef.current, loading: true, error: null };
    notify();
    doFetch();

    const timer = setInterval(doFetch, intervalMs);
    return () => {
      controller.abort();
      clearInterval(timer);
    };
  }, [url, intervalMs, notify]);

  return state;
}
