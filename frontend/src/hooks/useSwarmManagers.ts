import { usePollingFetch } from './usePollingFetch';
import type { SwarmManagers } from '../types/stats';

/** How often an open managers panel asks the swarm again. */
const MANAGERS_POLL_MS = 5_000;

/**
 * Raft state of the swarm's managers, read live from the swarm while
 * `active`. Unlike the graph, which keeps its last snapshot, it fails when
 * the managers can't answer, which is how a lost quorum shows.
 */
export function useSwarmManagers(active: boolean) {
  return usePollingFetch<SwarmManagers>(active ? '/api/swarm/managers' : null, MANAGERS_POLL_MS);
}
