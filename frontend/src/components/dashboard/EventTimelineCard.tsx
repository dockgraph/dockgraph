import { memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { STATUS_COLORS } from "./palette";
import { useRecentEvents, type DockerEvent } from "../../hooks/useRecentEvents";

const ACTION_COLORS: Record<string, string> = {
  start: STATUS_COLORS.green,
  stop: STATUS_COLORS.red,
  create: STATUS_COLORS.blue,
  destroy: STATUS_COLORS.red,
  die: STATUS_COLORS.red,
  connect: STATUS_COLORS.purple,
  disconnect: STATUS_COLORS.amber,
  kill: STATUS_COLORS.red,
  pause: STATUS_COLORS.amber,
  unpause: STATUS_COLORS.green,
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
    <DashboardCard title="Events" emptyMessage={events.length === 0 ? "No recent events" : undefined}>
      <div style={{
        display: "flex",
        flexDirection: "column",
        gap: 1,
        maxHeight: 220,
        overflowY: "auto",
      }}>
        {events.map((e: DockerEvent, i: number) => {
          const actionColor = ACTION_COLORS[e.action] ?? theme.nodeSubtext;
          return (
            <div
              key={`${e.timestamp}-${e.name}-${e.action}-${i}`}
              style={{
                display: "grid",
                gridTemplateColumns: "64px 72px 1fr",
                gap: 8,
                padding: "4px 6px",
                borderRadius: 3,
                fontSize: 11,
                alignItems: "center",
              }}
            >
              <span style={{ color: theme.nodeSubtext, fontFamily: "monospace", fontSize: 10 }}>
                {formatTime(e.timestamp)}
              </span>
              <span style={{
                color: actionColor,
                fontWeight: 600,
                fontSize: 10,
                textTransform: "uppercase",
                letterSpacing: "0.02em",
              }}>
                {e.action}
              </span>
              <span style={{
                color: theme.nodeText,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}>
                {e.name}
                <span style={{ color: theme.nodeSubtext, marginLeft: 6 }}>{e.type}</span>
              </span>
            </div>
          );
        })}
      </div>
    </DashboardCard>
  );
});
