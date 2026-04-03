import { useCallback, useState } from 'react';
import type { ContainerStatsData, StatsMessage } from '../types/stats';

/**
 * Stores container stats received from the WebSocket.
 * Returns a map of container name -> stats and a handler for incoming messages.
 */
export function useContainerStats() {
  const [stats, setStats] = useState<Map<string, ContainerStatsData>>(new Map());

  const handleStatsMessage = useCallback((data: StatsMessage) => {
    const next = new Map<string, ContainerStatsData>();
    for (const [name, s] of Object.entries(data.stats)) {
      next.set(name, s);
    }
    setStats(next);
  }, []);

  return { stats, handleStatsMessage };
}
