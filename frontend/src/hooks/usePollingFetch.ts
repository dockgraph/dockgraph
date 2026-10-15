import { useCallback, useEffect, useMemo, useRef } from "react";
import { useSyncExternalStore } from "react";

interface FetchState<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
}

/** Fetch state tagged with the URL it belongs to. */
interface KeyedState<T> extends FetchState<T> {
  url: string | null;
}

const EMPTY: KeyedState<never> = { url: null, data: null, loading: false, error: null };

/**
 * Polls `url`, starting the next request `intervalMs` after the previous one
 * settled, so requests never overlap and an older response can't overwrite
 * a newer one. Data from the last successful poll is kept
 * while the next one loads and after it fails, but only for the same URL:
 * results are keyed by URL, so a change of URL (another stack, scope or
 * range) never renders the previous URL's data, even for the one render
 * before the new fetch starts.
 */
export function usePollingFetch<T>(url: string | null, intervalMs: number): FetchState<T> {
  const stateRef = useRef<KeyedState<T>>(EMPTY as KeyedState<T>);
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
      if (stateRef.current !== (EMPTY as KeyedState<T>)) {
        stateRef.current = EMPTY as KeyedState<T>;
        notify();
      }
      return;
    }

    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;

    const doFetch = () => {
      fetch(url, { signal: controller.signal })
        .then(res => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
          return res.json();
        })
        .then((data: T) => {
          if (!controller.signal.aborted) {
            stateRef.current = { url, data, loading: false, error: null };
            notify();
          }
        })
        .catch(err => {
          if (!controller.signal.aborted) {
            stateRef.current = { ...stateRef.current, loading: false, error: err.message };
            notify();
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) timer = setTimeout(doFetch, intervalMs);
        });
    };

    // Keep data only when it belongs to this URL.
    const keep = stateRef.current.url === url ? stateRef.current.data : null;
    stateRef.current = { url, data: keep, loading: true, error: null };
    notify();
    doFetch();

    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [url, intervalMs, notify]);

  return useMemo<FetchState<T>>(() => {
    if (state.url === url) return { data: state.data, loading: state.loading, error: state.error };
    // Render before the effect for a new URL has run: nothing for it yet.
    return { data: null, loading: Boolean(url), error: null };
  }, [state, url]);
}
