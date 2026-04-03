import { memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { useRecentEvents, type DockerEvent } from "../../hooks/useRecentEvents";

const ACTION_COLORS: Record<string, string> = {
  start: "#22c55e",
  stop: "#ef4444",
  create: "#3b82f6",
  destroy: "#ef4444",
  die: "#ef4444",
  connect: "#8b5cf6",
  disconnect: "#f59e0b",
};

function formatTime(timestamp: string): string {
  const d = new Date(timestamp);
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

export const EventTimelineCard = memo(function EventTimelineCard() {
  const { theme } = useTheme();
  const { data } = useRecentEvents(50);
  const events = data?.events ?? [];

  return (
    <DashboardCard title="Event Timeline">
      <div style={{
        display: "flex",
        flexDirection: "column",
        gap: 4,
        maxHeight: 240,
        overflowY: "auto",
        fontSize: 12,
      }}>
        {events.length === 0 && (
          <span style={{ color: theme.nodeSubtext, padding: 8 }}>No recent events</span>
        )}
        {events.map((e: DockerEvent, i: number) => (
          <div key={`${e.timestamp}-${e.name}-${e.action}-${i}`} style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ color: theme.nodeSubtext, fontFamily: "monospace", flexShrink: 0, fontSize: 11 }}>
              {formatTime(e.timestamp)}
            </span>
            <span style={{
              color: ACTION_COLORS[e.action] ?? theme.nodeText,
              fontWeight: 500,
              minWidth: 70,
            }}>
              {e.action}
            </span>
            <span style={{ color: theme.nodeSubtext }}>{e.type}</span>
            <span style={{ color: theme.nodeText }}>{e.name}</span>
          </div>
        ))}
      </div>
    </DashboardCard>
  );
});
