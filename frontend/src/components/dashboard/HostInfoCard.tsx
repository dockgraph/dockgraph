import { memo } from "react";
import { useTheme } from "../../theme";
import { DashboardCard } from "./DashboardCard";
import { useSystemInfo } from "../../hooks/useSystemInfo";
import { formatBytes } from "../../utils/format";

export const HostInfoCard = memo(function HostInfoCard() {
  const { theme } = useTheme();
  const { data } = useSystemInfo();

  if (!data) {
    return (
      <DashboardCard title="Docker Host">
        <span style={{ fontSize: 12, color: theme.nodeSubtext }}>Loading...</span>
      </DashboardCard>
    );
  }

  const rows = [
    ["Version", data.dockerVersion],
    ["OS", `${data.os} (${data.arch})`],
    ["Kernel", data.kernel],
    ["Storage", data.storageDriver],
    ["CPUs", String(data.cpus)],
    ["Memory", formatBytes(data.memTotal)],
    ["Cgroup", `v${data.cgroupVersion}`],
  ];

  return (
    <DashboardCard title="Docker Host">
      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        {rows.map(([label, value]) => (
          <div key={label} style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
            <span style={{ color: theme.nodeSubtext }}>{label}</span>
            <span style={{ color: theme.nodeText, fontFamily: "monospace" }}>{value}</span>
          </div>
        ))}
      </div>
    </DashboardCard>
  );
});
