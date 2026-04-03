import { memo } from "react";
import { useTableSort } from "../../hooks/useTableSort";
import { useTableGrouping } from "../../hooks/useTableGrouping";
import { networkColor } from "../../utils/colors";
import { ContainerRow } from "./ContainerRow";
import { GroupedTable } from "./GroupedTable";
import { CONTAINER_GRID } from "./tableStyles";
import type { GroupOption } from "./TableToolbar";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

const GROUP_OPTIONS: GroupOption[] = [
  { key: "compose", label: "Compose Project" },
  { key: "network", label: "Network" },
  { key: "status", label: "Status" },
  { key: "none", label: "None" },
];

const COLUMNS = [
  { key: "name", label: "Name" },
  { key: "image", label: "Image" },
  { key: "status", label: "Status" },
  { key: "ports", label: "Ports" },
  { key: "network", label: "Network" },
];

interface Props {
  nodes: DGNode[];
  statsMap: Map<string, ContainerStatsData>;
  selectedNodeId: string | null;
  onRowClick: (nodeId: string) => void;
}

export const ContainerTable = memo(function ContainerTable({
  nodes,
  statsMap,
  selectedNodeId,
  onRowClick,
}: Props) {
  const sort = useTableSort<string>("name");
  const grouping = useTableGrouping(nodes);

  return (
    <GroupedTable
      nodes={nodes}
      grouping={grouping}
      sort={sort}
      columns={COLUMNS}
      gridTemplate={CONTAINER_GRID}
      groupOptions={GROUP_OPTIONS}
      emptyMessage="No containers found"
      groupColor={(group) =>
        group.key.startsWith("network:") ? networkColor(group.label) : undefined
      }
      sortKeyFn={(n) => ({
        name: n.name,
        image: n.image ?? "",
        status: n.status ?? "",
        ports: n.ports?.map((p) => `${p.host}:${p.container}`).join(",") ?? "",
        network: n.networkId?.replace(/^network:/, "") ?? "",
      })}
      renderRow={(node) => (
        <ContainerRow
          key={node.id}
          node={node}
          stats={statsMap.get(node.name)}
          selected={node.id === selectedNodeId}
          onClick={onRowClick}
          gridTemplate={CONTAINER_GRID}
        />
      )}
    />
  );
});
