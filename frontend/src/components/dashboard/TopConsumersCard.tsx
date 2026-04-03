import { useMemo, useState, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { formatBytes, formatPercent } from "../../utils/format";
import type { ContainerStatsData } from "../../types/stats";

interface Props {
  statsMap: Map<string, ContainerStatsData>;
}

type SortKey = "cpu" | "mem" | "net" | "disk";

interface Row {
  name: string;
  cpu: number;
  mem: number;
  net: number;
  disk: number;
}

export const TopConsumersCard = memo(function TopConsumersCard({ statsMap }: Props) {
  const { theme } = useTheme();
  const [sortBy, setSortBy] = useState<SortKey>("cpu");

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
    { key: "mem", label: "Memory", format: formatBytes },
    { key: "net", label: "Net I/O", format: formatBytes },
    { key: "disk", label: "Disk I/O", format: formatBytes },
  ];

  return (
    <DashboardCard title="Top Consumers">
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
        <thead>
          <tr>
            <th style={{ textAlign: "left", padding: "4px 8px", color: theme.nodeSubtext, fontWeight: 500 }}>Name</th>
            {columns.map(col => (
              <th
                key={col.key}
                onClick={() => setSortBy(col.key)}
                style={{
                  textAlign: "right",
                  padding: "4px 8px",
                  color: sortBy === col.key ? theme.nodeText : theme.nodeSubtext,
                  fontWeight: sortBy === col.key ? 600 : 500,
                  cursor: "pointer",
                }}
              >
                {col.label}{sortBy === col.key ? " \u25bc" : ""}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map(row => (
            <tr key={row.name}>
              <td style={{ padding: "4px 8px", color: theme.nodeText }}>{row.name}</td>
              {columns.map(col => (
                <td key={col.key} style={{ textAlign: "right", padding: "4px 8px", color: theme.nodeText, fontFamily: "monospace" }}>
                  {col.format(row[col.key])}
                </td>
              ))}
            </tr>
          ))}
          {rows.length === 0 && (
            <tr>
              <td colSpan={5} style={{ padding: 16, textAlign: "center", color: theme.nodeSubtext }}>
                No stats available
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </DashboardCard>
  );
});
