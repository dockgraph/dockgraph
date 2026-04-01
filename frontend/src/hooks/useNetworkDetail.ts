import { useEffect, useRef, useState } from 'react';
import type { NetworkDetail } from '../types/stats';

interface DetailResult {
  data: NetworkDetail | null;
  loading: boolean;
  error: string | null;
}

/**
 * Fetches network inspect data when the network name changes.
 * Cancels in-flight requests on name change or unmount.
 */
export function useNetworkDetail(networkName: string | null): DetailResult {
  const [state, setState] = useState<DetailResult>({ data: null, loading: false, error: null });
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!networkName) {
      setState({ data: null, loading: false, error: null });
      return;
    }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setState((prev) => ({ ...prev, loading: true, error: null }));

    fetch(`/api/networks/${encodeURIComponent(networkName)}`, { signal: controller.signal })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: NetworkDetail) => {
        if (!controller.signal.aborted) {
          setState({ data, loading: false, error: null });
        }
      })
      .catch((err) => {
        if (!controller.signal.aborted) {
          setState({ data: null, loading: false, error: err.message });
        }
      });

    return () => controller.abort();
  }, [networkName]);

  return state;
}
