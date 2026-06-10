import { useMemo, useState, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { ProgressBar } from "./ProgressBar";
import { METRIC_COLORS } from "./palette";
import { formatBytes, formatPercent } from "../../utils/format";
import type { ContainerStatsData } from "../../types/stats";

interface Props {
  statsMap: Map<string, ContainerStatsData>;
  /** Open the detail panel for the clicked container. */
  onInspect: (nodeId: string) => void;
}

type SortKey = "cpu" | "mem" | "net" | "disk";

interface Row {
  name: string;
  cpu: number;
  mem: number;
  net: number;
  disk: number;
}

export const TopConsumersCard = memo(function TopConsumersCard({ statsMap, onInspect }: Props) {
  const { theme } = useTheme();
  const [sortBy, setSortBy] = useState<SortKey>("cpu");
  const [hovered, setHovered] = useState<string | null>(null);

  const rows = useMemo(() => {
    const entries: Row[] = Array.from(statsMap.entries()).map(([name, s]) => ({
      name,
      cpu: s.cpuPercent,
      mem: s.memUsage,
      net: s.netRx + s.netTx,
      disk: s.blockRead + s.blockWrite,
    }));
    entries.sort((a, b) => b[sortBy] - a[sortBy]);
    return entries.slice(0, 5);
  }, [statsMap, sortBy]);

  const columns: { key: SortKey; label: string; format: (v: number) => string }[] = [
    { key: "cpu", label: "CPU", format: formatPercent },
    { key: "mem", label: "Mem", format: formatBytes },
    { key: "net", label: "Net", format: formatBytes },
    { key: "disk", label: "Disk", format: formatBytes },
  ];

  const maxVal = rows.length > 0 ? Math.max(...rows.map(r => r[sortBy]), 1) : 1;

  return (
    <DashboardCard title="Top Consumers" emptyMessage={rows.length === 0 ? "No stats available" : undefined}>
      {/* Column headers */}
      <div style={{
        display: "grid",
        gridTemplateColumns: "1.5fr repeat(4, 1fr)",
        gap: 4,
        padding: "0 0 6px",
        borderBottom: `1px solid ${theme.panelBorder}`,
        marginBottom: 4,
      }}>
        <span style={{ fontFamily: "var(--dg-font-mono)", fontSize: 10, color: theme.nodeSubtext, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.04em" }}>
          Container
        </span>
        {columns.map(col => (
          <span
            key={col.key}
            onClick={() => setSortBy(col.key)}
            style={{
              fontFamily: "var(--dg-font-mono)",
              fontSize: 10,
              textAlign: "right",
              color: sortBy === col.key ? theme.accent : theme.nodeSubtext,
              fontWeight: 600,
              textTransform: "uppercase",
              letterSpacing: "0.04em",
              cursor: "pointer",
              transition: "color 0.15s",
            }}
          >
            {col.label}{sortBy === col.key ? " \u25bc" : ""}
          </span>
        ))}
      </div>

      {/* Rows */}
      <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {rows.map(row => (
          <div
            key={row.name}
            role="button"
            tabIndex={0}
            onClick={() => onInspect(`container:${row.name}`)}
            onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onInspect(`container:${row.name}`); } }}
            onMouseEnter={() => setHovered(row.name)}
            onMouseLeave={() => setHovered(null)}
            title={`Inspect ${row.name}`}
            style={{
              cursor: "pointer",
              borderRadius: 5,
              margin: "0 -6px",
              padding: "0 6px",
              background: hovered === row.name ? theme.rowHover : "transparent",
              transition: "background 0.12s",
            }}
          >
            <div style={{
              display: "grid",
              gridTemplateColumns: "1.5fr repeat(4, 1fr)",
              gap: 4,
              padding: "5px 0",
              alignItems: "center",
            }}>
              <span style={{
                fontFamily: "var(--dg-font-mono)",
                fontSize: 12,
                color: theme.nodeText,
                fontWeight: 500,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}>
                {row.name}
              </span>
              {columns.map(col => (
                <span key={col.key} style={{
                  textAlign: "right",
                  fontSize: 11,
                  fontFamily: "var(--dg-font-mono)",
                  color: sortBy === col.key ? METRIC_COLORS[col.key] : theme.nodeText,
                }}>
                  {col.format(row[col.key])}
                </span>
              ))}
            </div>
            <ProgressBar
              percent={(row[sortBy] / maxVal) * 100}
              color={METRIC_COLORS[sortBy]}
              height={2}
            />
          </div>
        ))}
      </div>
    </DashboardCard>
  );
});
