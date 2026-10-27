import { memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { useSystemInfo } from "../../hooks/useSystemInfo";
import { formatBytes } from "../../utils/format";

interface Props {
  /** Swarm node the figures belong to (see DashboardCard). */
  node?: string;
}

export const HostInfoCard = memo(function HostInfoCard({ node }: Props) {
  const { theme } = useTheme();
  const { data } = useSystemInfo();

  if (!data) {
    return <DashboardCard title="Docker Host" node={node} loading />;
  }

  const rows: [string, string][] = [
    ["Docker", data.dockerVersion],
    ["OS", data.os],
    ["Arch", data.arch],
    ["Kernel", data.kernel],
    ["Storage", data.storageDriver],
    ["CPUs", String(data.cpus)],
    ["Memory", formatBytes(data.memTotal)],
  ];

  return (
    <DashboardCard title="Docker Host" node={node}>
      <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
        {rows.map(([label, value]) => (
          <div key={label} style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
            <span style={{ fontSize: 11, color: theme.nodeSubtext }}>{label}</span>
            <span style={{
              fontSize: 11,
              color: theme.nodeText,
              fontFamily: "var(--dg-font-mono)",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              maxWidth: "60%",
              textAlign: "right",
            }}>{value}</span>
          </div>
        ))}
      </div>
    </DashboardCard>
  );
});
