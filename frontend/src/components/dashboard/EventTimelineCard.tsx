import { useState, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { STATUS_COLORS } from "./palette";
import { useRecentEvents, type DockerEvent } from "../../hooks/useRecentEvents";
import type { DGNode } from "../../types";

interface Props {
  nodes: DGNode[];
  /** Open the detail panel for the resource the event refers to. */
  onInspect: (nodeId: string) => void;
}

/** Event resource types that map to an inspectable graph node. */
const INSPECTABLE_TYPES = new Set(["container", "network", "volume"]);

/**
 * Resolve an event's resource reference to a live graph node id, or null when
 * it has no counterpart (e.g. image events, or a resource already removed).
 * Falls back to compose-style suffix matching, mirroring panel navigation.
 */
function resolveNodeId(nodes: DGNode[], type: string, name: string): string | null {
  if (!INSPECTABLE_TYPES.has(type)) return null;
  const exactId = `${type}:${name}`;
  if (nodes.some((n) => n.id === exactId)) return exactId;
  const match = nodes.find(
    (n) =>
      n.type === type &&
      (n.name === name || n.name.endsWith(`-${name}`) || n.name.endsWith(`_${name}`)),
  );
  return match ? match.id : null;
}

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

export const EventTimelineCard = memo(function EventTimelineCard({ nodes, onInspect }: Props) {
  const { theme } = useTheme();
  const { data } = useRecentEvents(50);
  const events = data?.events ?? [];
  const [hovered, setHovered] = useState<number | null>(null);

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
          const nodeId = resolveNodeId(nodes, e.type, e.name);
          const interactive = nodeId !== null;
          return (
            <div
              key={`${e.timestamp}-${e.name}-${e.action}-${i}`}
              role={interactive ? "button" : undefined}
              tabIndex={interactive ? 0 : undefined}
              onClick={interactive ? () => onInspect(nodeId) : undefined}
              onKeyDown={interactive ? (ev) => { if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); onInspect(nodeId); } } : undefined}
              onMouseEnter={interactive ? () => setHovered(i) : undefined}
              onMouseLeave={interactive ? () => setHovered(null) : undefined}
              title={interactive ? `Inspect ${e.name}` : undefined}
              style={{
                display: "grid",
                gridTemplateColumns: "64px 72px 1fr",
                gap: 8,
                padding: "4px 6px",
                borderRadius: 3,
                fontSize: 11,
                alignItems: "center",
                cursor: interactive ? "pointer" : "default",
                background: hovered === i ? theme.rowHover : "transparent",
                transition: "background 0.12s",
              }}
            >
              <span style={{ color: theme.nodeSubtext, fontFamily: "var(--dg-font-mono)", fontSize: 10 }}>
                {formatTime(e.timestamp)}
              </span>
              <span style={{
                fontFamily: "var(--dg-font-mono)",
                color: actionColor,
                fontWeight: 600,
                fontSize: 10,
                textTransform: "uppercase",
                letterSpacing: "0.02em",
              }}>
                {e.action}
              </span>
              <span style={{
                fontFamily: "var(--dg-font-mono)",
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
