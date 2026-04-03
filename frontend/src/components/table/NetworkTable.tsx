import { memo, useMemo } from "react";
import { useTableSort } from "../../hooks/useTableSort";
import { useTableGrouping } from "../../hooks/useTableGrouping";
import { NetworkRow } from "./NetworkRow";
import { GroupedTable } from "./GroupedTable";
import { NETWORK_GRID } from "./tableStyles";
import type { GroupOption } from "./TableToolbar";
import type { DGNode, DGEdge } from "../../types";

const GROUP_OPTIONS: GroupOption[] = [
  { key: "compose", label: "Compose Project" },
  { key: "driver", label: "Driver" },
  { key: "none", label: "None" },
];

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
  const sort = useTableSort<string>("name");
  const grouping = useTableGrouping(nodes, "compose");

  const containerCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const n of allNodes) {
      if (n.type === "container" && n.networkId) {
        counts.set(n.networkId, (counts.get(n.networkId) ?? 0) + 1);
      }
    }
    for (const e of edges) {
      if (e.type === "secondary_network") {
        counts.set(e.target, (counts.get(e.target) ?? 0) + 1);
      }
    }
    return counts;
  }, [allNodes, edges]);

  return (
    <GroupedTable
      nodes={nodes}
      grouping={grouping}
      sort={sort}
      columns={COLUMNS}
      gridTemplate={NETWORK_GRID}
      groupOptions={GROUP_OPTIONS}
      emptyMessage="No networks found"
      sortKeyFn={(n) => ({
        name: n.name,
        driver: n.driver ?? "",
        subnet: n.subnet ?? "",
        gateway: n.gateway ?? "",
        containers: containerCounts.get(n.id) ?? 0,
      })}
      renderRow={(node) => (
        <NetworkRow
          key={node.id}
          node={node}
          containerCount={containerCounts.get(node.id) ?? 0}
          selected={node.id === selectedNodeId}
          onClick={onRowClick}
          gridTemplate={NETWORK_GRID}
        />
      )}
    />
  );
});
