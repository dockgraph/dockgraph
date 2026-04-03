import { usePollingFetch } from "./usePollingFetch";

export interface ImagesData {
  total: number;
  totalSize: number;
  dangling: number;
  danglingSize: number;
  uniqueTags: number;
}

export function useImages() {
  return usePollingFetch<ImagesData>("/api/images", 60_000);
}
