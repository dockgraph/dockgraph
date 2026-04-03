import { useState, useMemo, memo } from "react";
import { useTheme } from "../../theme";
import { ContainerTable } from "./ContainerTable";
import { NetworkTable } from "./NetworkTable";
import { VolumeTable } from "./VolumeTable";
import { tableLayout, resourceTabs } from "./tableStyles";
import type { DGNode, DGEdge } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

type ResourceTab = "containers" | "networks" | "volumes";

const TABS: { key: ResourceTab; label: string }[] = [
  { key: "containers", label: "Containers" },
  { key: "networks", label: "Networks" },
  { key: "volumes", label: "Volumes" },
];

function tabForSelection(id: string | null): ResourceTab | null {
  if (!id) return null;
  if (id.startsWith("network:")) return "networks";
  if (id.startsWith("volume:")) return "volumes";
  if (id.startsWith("container:")) return "containers";
  return null;
}

interface Props {
  nodes: DGNode[];
  edges: DGEdge[];
  statsMap: Map<string, ContainerStatsData>;
  matchingNodeIds: Set<string> | null;
  selectedNodeId: string | null;
  onRowClick: (nodeId: string) => void;
}

export const TableView = memo(function TableView({
  nodes,
  edges,
  statsMap,
  matchingNodeIds,
  selectedNodeId,
  onRowClick,
}: Props) {
  const { theme } = useTheme();
  const [activeTab, setActiveTab] = useState<ResourceTab>("containers");
  const [prevSelection, setPrevSelection] = useState<string | null>(null);

  // Auto-switch tab when selection changes to a different resource type.
  // Uses the "setState during render" pattern documented by React for
  // adjusting state based on changed props without an extra render pass.
  if (selectedNodeId !== prevSelection) {
    setPrevSelection(selectedNodeId);
    const target = tabForSelection(selectedNodeId);
    if (target && target !== activeTab) setActiveTab(target);
  }

  const styles = tableLayout(theme);
  const tabStyles = resourceTabs(theme);

  const filtered = useMemo(
    () => matchingNodeIds ? nodes.filter((n) => matchingNodeIds.has(n.id)) : nodes,
    [nodes, matchingNodeIds],
  );

  const containers = useMemo(() => filtered.filter((n) => n.type === "container"), [filtered]);
  const networks = useMemo(() => filtered.filter((n) => n.type === "network"), [filtered]);
  const volumes = useMemo(() => filtered.filter((n) => n.type === "volume"), [filtered]);

  const counts: Record<ResourceTab, number> = {
    containers: containers.length,
    networks: networks.length,
    volumes: volumes.length,
  };

  return (
    <div style={styles.container}>
      <div style={tabStyles.container}>
        {TABS.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            style={tabStyles.tab(activeTab === tab.key)}
          >
            {tab.label}
            <span style={{
              marginLeft: 6,
              fontSize: 11,
              color: theme.nodeSubtext,
              opacity: 0.7,
            }}>
              {counts[tab.key]}
            </span>
          </button>
        ))}
      </div>

      {activeTab === "containers" && (
        <ContainerTable
          nodes={containers}
          statsMap={statsMap}
          selectedNodeId={selectedNodeId}
          onRowClick={onRowClick}
        />
      )}
      {activeTab === "networks" && (
        <NetworkTable
          nodes={networks}
          allNodes={nodes}
          edges={edges}
          selectedNodeId={selectedNodeId}
          onRowClick={onRowClick}
        />
      )}
      {activeTab === "volumes" && (
        <VolumeTable
          nodes={volumes}
          edges={edges}
          selectedNodeId={selectedNodeId}
          onRowClick={onRowClick}
        />
      )}
    </div>
  );
});
