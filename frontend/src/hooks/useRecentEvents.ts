import { usePollingFetch } from "./usePollingFetch";

export interface DockerEvent {
  timestamp: string;
  action: string;
  type: string;
  name: string;
  attributes?: Record<string, string>;
}

interface EventsResponse {
  events: DockerEvent[];
}

export function useRecentEvents(limit = 50) {
  return usePollingFetch<EventsResponse>(`/api/events/recent?limit=${limit}`, 10_000);
}
