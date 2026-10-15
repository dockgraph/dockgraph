import { usePollingFetch } from "./usePollingFetch";

export interface SystemInfo {
  dockerVersion: string;
  os: string;
  arch: string;
  kernel: string;
  storageDriver: string;
  cpus: number;
  memTotal: number;
  cgroupVersion: string;
  /** Resolved runtime mode: "standalone", "swarm" or "agent". */
  mode?: string;
  /** Present when the daemon is an active swarm member. */
  swarm?: { nodeId: string; managers: number; nodes: number } | null;
}

export function useSystemInfo() {
  return usePollingFetch<SystemInfo>("/api/system/info", 300_000);
}
