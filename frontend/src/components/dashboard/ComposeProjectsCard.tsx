import { useMemo, memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { ProgressBar } from "./ProgressBar";
import { STATUS_COLORS } from "./palette";
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
    <DashboardCard title="Compose Projects" emptyMessage={projects.length === 0 ? "No containers" : undefined}>
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        {projects.map(p => {
          const allRunning = p.running === p.total;
          const allStopped = p.running === 0;
          const statusColor = allRunning ? STATUS_COLORS.green : allStopped ? STATUS_COLORS.red : STATUS_COLORS.amber;
          const pct = p.total > 0 ? (p.running / p.total) * 100 : 0;

          return (
            <div key={p.name}>
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                <span style={{
                  width: 7,
                  height: 7,
                  borderRadius: "50%",
                  background: statusColor,
                  flexShrink: 0,
                }} />
                <span style={{
                  fontSize: 12,
                  color: theme.nodeText,
                  fontWeight: 500,
                  flex: 1,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                }}>
                  {p.name}
                </span>
                <span style={{ fontSize: 11, color: theme.nodeSubtext, fontFamily: "monospace", flexShrink: 0 }}>
                  {p.running}/{p.total}
                </span>
              </div>
              <ProgressBar percent={pct} color={statusColor} />
            </div>
          );
        })}
      </div>
    </DashboardCard>
  );
});
