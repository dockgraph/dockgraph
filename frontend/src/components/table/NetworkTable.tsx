import { memo, useMemo } from "react";
import { useTheme } from "../../theme";
import { useTableSort } from "../../hooks/useTableSort";
import { SortableHeader } from "./SortableHeader";
import { NetworkRow } from "./NetworkRow";
import { NETWORK_GRID, tableLayout } from "./tableStyles";
import type { DGNode, DGEdge } from "../../types";

const COLUMNS = [
  { key: "name", label: "Name" },
  { key: "driver", label: "Driver" },
  { key: "subnet", label: "Subnet" },
  { key: "gateway", label: "Gateway" },
  { key: "containers", label: "Containers" },
];

interface Props {
  nodes: DGNode[];
  allNodes: DGNode[];
  edges: DGEdge[];
  selectedNodeId: string | null;
  onRowClick: (nodeId: string) => void;
}

export const NetworkTable = memo(function NetworkTable({
  nodes,
  allNodes,
  edges,
  selectedNodeId,
  onRowClick,
}: Props) {
  const { theme } = useTheme();
  const sort = useTableSort<string>("name");

  const containerCounts = useMemo(() => {
    const counts = new Map<string, number>();
    // Primary network membership.
    for (const n of allNodes) {
      if (n.type === "container" && n.networkId) {
        counts.set(n.networkId, (counts.get(n.networkId) ?? 0) + 1);
      }
    }
    // Secondary network edges (container → network).
    for (const e of edges) {
      if (e.type === "secondary_network") {
        counts.set(e.target, (counts.get(e.target) ?? 0) + 1);
      }
    }
    return counts;
  }, [allNodes, edges]);

  const sorted = sort.sortItems(nodes, (n) => ({
    name: n.name,
    driver: n.driver ?? "",
    subnet: n.subnet ?? "",
    gateway: n.gateway ?? "",
    containers: containerCounts.get(n.id) ?? 0,
  }));

  const layout = tableLayout(theme);

  return (
    <div style={{ ...layout.scrollBody, paddingTop: 12 }}>
      <div style={layout.card}>
        <SortableHeader
          columns={COLUMNS}
          sortColumn={sort.column}
          sortDirection={sort.direction}
          onSort={sort.toggleSort}
          gridTemplate={NETWORK_GRID}
        />
        {sorted.map((node) => (
          <NetworkRow
            key={node.id}
            node={node}
            containerCount={containerCounts.get(node.id) ?? 0}
            selected={node.id === selectedNodeId}
            onClick={onRowClick}
            gridTemplate={NETWORK_GRID}
          />
        ))}
        {nodes.length === 0 && (
          <div style={{ padding: 32, textAlign: "center", color: theme.nodeSubtext, fontSize: 13 }}>
            No networks found
          </div>
        )}
      </div>
    </div>
  );
});
