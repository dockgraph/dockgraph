import { usePollingFetch } from "./usePollingFetch";

export interface DiskCategory {
  total: number;
  count: number;
  reclaimable: number;
}

export interface DiskUsageData {
  images: DiskCategory;
  containers: DiskCategory;
  volumes: DiskCategory;
  buildCache: DiskCategory;
}

export function useDiskUsage() {
  return usePollingFetch<DiskUsageData>("/api/system/disk-usage", 60_000);
}
