import { memo, useMemo } from "react";
import { useTableSort } from "../../hooks/useTableSort";
import { useTableGrouping } from "../../hooks/useTableGrouping";
import { VolumeRow } from "./VolumeRow";
import { GroupedTable } from "./GroupedTable";
import { VOLUME_GRID } from "./tableStyles";
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
  { key: "mountpoint", label: "Mounted At" },
  { key: "containers", label: "Containers" },
];

interface Props {
  nodes: DGNode[];
  edges: DGEdge[];
  selectedNodeId: string | null;
  onRowClick: (nodeId: string) => void;
}

export const VolumeTable = memo(function VolumeTable({
  nodes,
  edges,
  selectedNodeId,
  onRowClick,
}: Props) {
  const sort = useTableSort<string>("name");
  const grouping = useTableGrouping(nodes, "compose");

  const { containerCounts, mountPaths } = useMemo(() => {
    const counts = new Map<string, number>();
    const paths = new Map<string, string>();
    for (const e of edges) {
      if (e.type === "volume_mount") {
        counts.set(e.source, (counts.get(e.source) ?? 0) + 1);
        if (!paths.has(e.source) && e.mountPath) {
          paths.set(e.source, e.mountPath);
        }
      }
    }
    return { containerCounts: counts, mountPaths: paths };
  }, [edges]);

  return (
    <GroupedTable
      nodes={nodes}
      grouping={grouping}
      sort={sort}
      columns={COLUMNS}
      gridTemplate={VOLUME_GRID}
      groupOptions={GROUP_OPTIONS}
      emptyMessage="No volumes found"
      sortKeyFn={(n) => ({
        name: n.name,
        driver: n.driver ?? "",
        mountpoint: mountPaths.get(n.id) ?? "",
        containers: containerCounts.get(n.id) ?? 0,
      })}
      renderRow={(node) => (
        <VolumeRow
          key={node.id}
          node={node}
          mountPath={mountPaths.get(node.id) ?? ""}
          containerCount={containerCounts.get(node.id) ?? 0}
          selected={node.id === selectedNodeId}
          onClick={onRowClick}
          gridTemplate={VOLUME_GRID}
        />
      )}
    />
  );
});
