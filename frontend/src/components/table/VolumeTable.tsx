import { memo, useMemo } from "react";
import { useTheme } from "../../theme";
import { useTableSort } from "../../hooks/useTableSort";
import { SortableHeader } from "./SortableHeader";
import { VolumeRow } from "./VolumeRow";
import { VOLUME_GRID, tableLayout } from "./tableStyles";
import type { DGNode, DGEdge } from "../../types";

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
  const { theme } = useTheme();
  const sort = useTableSort<string>("name");

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

  const sorted = sort.sortItems(nodes, (n) => ({
    name: n.name,
    driver: n.driver ?? "",
    mountpoint: mountPaths.get(n.id) ?? "",
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
          gridTemplate={VOLUME_GRID}
        />
        {sorted.map((node) => (
          <VolumeRow
            key={node.id}
            node={node}
            mountPath={mountPaths.get(node.id) ?? ""}
            containerCount={containerCounts.get(node.id) ?? 0}
            selected={node.id === selectedNodeId}
            onClick={onRowClick}
            gridTemplate={VOLUME_GRID}
          />
        ))}
        {nodes.length === 0 && (
          <div style={{ padding: 32, textAlign: "center", color: theme.nodeSubtext, fontSize: 13 }}>
            No volumes found
          </div>
        )}
      </div>
    </div>
  );
});
