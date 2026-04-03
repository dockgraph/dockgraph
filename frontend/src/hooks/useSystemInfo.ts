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
}

export function useSystemInfo() {
  return usePollingFetch<SystemInfo>("/api/system/info", 300_000);
}
