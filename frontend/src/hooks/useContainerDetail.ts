import { useEffect, useRef, useState } from 'react';
import type { ContainerDetail } from '../types/stats';

interface DetailResult {
  data: ContainerDetail | null;
  loading: boolean;
  error: string | null;
}

/**
 * Fetches container inspect data when the container ID changes.
 * Cancels in-flight requests on ID change or unmount.
 */
export function useContainerDetail(containerId: string | null): DetailResult {
  const [state, setState] = useState<DetailResult>({ data: null, loading: false, error: null });
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!containerId) {
      setState({ data: null, loading: false, error: null });
      return;
    }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setState((prev) => ({ ...prev, loading: true, error: null }));

    fetch(`/api/containers/${encodeURIComponent(containerId)}`, { signal: controller.signal })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: ContainerDetail) => {
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
  }, [containerId]);

  return state;
}
