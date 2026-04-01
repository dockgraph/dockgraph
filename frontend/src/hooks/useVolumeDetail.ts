import { useEffect, useRef, useState } from 'react';
import type { VolumeDetail } from '../types/stats';

interface DetailResult {
  data: VolumeDetail | null;
  loading: boolean;
  error: string | null;
}

/**
 * Fetches volume inspect data when the volume name changes.
 * Cancels in-flight requests on name change or unmount.
 */
export function useVolumeDetail(volumeName: string | null): DetailResult {
  const [state, setState] = useState<DetailResult>({ data: null, loading: false, error: null });
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!volumeName) {
      setState({ data: null, loading: false, error: null });
      return;
    }

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setState((prev) => ({ ...prev, loading: true, error: null }));

    fetch(`/api/volumes/${encodeURIComponent(volumeName)}`, { signal: controller.signal })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: VolumeDetail) => {
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
  }, [volumeName]);

  return state;
}
