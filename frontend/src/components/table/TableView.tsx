import { useMemo, useEffect, memo } from "react";
import { useTheme } from "../../theme";
import { ContainerTable } from "./ContainerTable";
import { NetworkTable } from "./NetworkTable";
import { VolumeTable } from "./VolumeTable";
import { tableLayout, resourceTabs } from "./tableStyles";
import type { DGNode, DGEdge } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

export type ResourceTab = "containers" | "services" | "networks" | "volumes";

const TABS: { key: ResourceTab; label: string }[] = [
  { key: "containers", label: "Containers" },
  { key: "services", label: "Services" },
  { key: "networks", label: "Networks" },
  { key: "volumes", label: "Volumes" },
];

function tabForSelection(id: string | null): ResourceTab | null {
  if (!id) return null;
  if (id.startsWith("network:")) return "networks";
  if (id.startsWith("volume:")) return "volumes";
  if (id.startsWith("container:")) return "containers";
  if (id.startsWith("service:")) return "services";
  return null;
}

interface Props {
  nodes: DGNode[];
  edges: DGEdge[];
  statsMap: Map<string, ContainerStatsData>;
  matchingNodeIds: Set<string> | null;
  selectedNodeId: string | null;
  onRowClick: (nodeId: string) => void;
  /** Active resource subtab — controlled by the parent so other views can drive it. */
  activeTab: ResourceTab;
  onTabChange: (tab: ResourceTab) => void;
}

export const TableView = memo(function TableView({
  nodes,
  edges,
  statsMap,
  matchingNodeIds,
  selectedNodeId,
  onRowClick,
  activeTab,
  onTabChange,
}: Props) {
  const { theme } = useTheme();

  // Follow the selection: when a resource is selected (e.g. from the graph or
  // a cross-reference), surface it by switching to its subtab.
  useEffect(() => {
    const target = tabForSelection(selectedNodeId);
    if (target) onTabChange(target);
  }, [selectedNodeId, onTabChange]);

  const styles = tableLayout(theme);
  const tabStyles = resourceTabs(theme);

  const filtered = useMemo(
    () => matchingNodeIds ? nodes.filter((n) => matchingNodeIds.has(n.id)) : nodes,
    [nodes, matchingNodeIds],
  );

  const containers = useMemo(() => filtered.filter((n) => n.type === "container"), [filtered]);
  const services = useMemo(() => filtered.filter((n) => n.type === "service"), [filtered]);
  const networks = useMemo(() => filtered.filter((n) => n.type === "network"), [filtered]);
  const volumes = useMemo(() => filtered.filter((n) => n.type === "volume"), [filtered]);

  // The Services tab only appears when the graph has swarm services (or it is
  // the active tab), so standalone setups keep the familiar three tabs.
  const hasServices = useMemo(() => nodes.some((n) => n.type === "service"), [nodes]);
  const visibleTabs = TABS.filter((t) => t.key !== "services" || hasServices || activeTab === "services");

  const counts: Record<ResourceTab, number> = {
    containers: containers.length,
    services: services.length,
    networks: networks.length,
    volumes: volumes.length,
  };

  return (
    <div style={styles.container}>
      <div style={tabStyles.container}>
        {visibleTabs.map((tab) => (
          <button
            key={tab.key}
            className={activeTab === tab.key ? "dg-resource-tab dg-resource-tab--active" : "dg-resource-tab"}
            onClick={() => onTabChange(tab.key)}
            style={tabStyles.tab(activeTab === tab.key)}
          >
            {tab.label}
            <span style={{
              marginLeft: 7,
              fontFamily: "var(--dg-font-mono)",
              fontSize: 11,
              color: theme.nodeSubtext,
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
      {activeTab === "services" && (
        <ContainerTable
          nodes={services}
          statsMap={statsMap}
          selectedNodeId={selectedNodeId}
          onRowClick={onRowClick}
          emptyMessage="No services found"
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
