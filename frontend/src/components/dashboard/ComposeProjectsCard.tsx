import { useMemo, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import type { DGNode } from "../../types";

interface Props {
  nodes: DGNode[];
}

interface ProjectGroup {
  name: string;
  total: number;
  running: number;
}

export const ComposeProjectsCard = memo(function ComposeProjectsCard({ nodes }: Props) {
  const { theme } = useTheme();

  const projects = useMemo(() => {
    const groups = new Map<string, ProjectGroup>();
    const containers = nodes.filter(n => n.type === "container");

    for (const c of containers) {
      const project = c.labels?.["com.docker.compose.project"] ?? "standalone";
      const group = groups.get(project) ?? { name: project, total: 0, running: 0 };
      group.total++;
      if (c.status === "running") group.running++;
      groups.set(project, group);
    }

    return Array.from(groups.values()).sort((a, b) => b.total - a.total);
  }, [nodes]);

  return (
    <DashboardCard title="Compose Projects">
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        {projects.length === 0 && (
          <span style={{ fontSize: 12, color: theme.nodeSubtext }}>No containers</span>
        )}
        {projects.map(p => {
          const allRunning = p.running === p.total;
          const allStopped = p.running === 0;
          const statusColor = allRunning ? "#22c55e" : allStopped ? "#ef4444" : "#f59e0b";
          const statusLabel = allRunning ? "all running" : allStopped ? "all stopped" : "partial";

          return (
            <div key={p.name} style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12 }}>
              <span style={{ width: 8, height: 8, borderRadius: "50%", background: statusColor, flexShrink: 0 }} />
              <span style={{ color: theme.nodeText, flex: 1, fontWeight: 500 }}>{p.name}</span>
              <span style={{ color: theme.nodeSubtext, fontFamily: "monospace" }}>{p.running}/{p.total}</span>
              <span style={{ color: theme.nodeSubtext, fontSize: 11 }}>{statusLabel}</span>
            </div>
          );
        })}
      </div>
    </DashboardCard>
  );
});
