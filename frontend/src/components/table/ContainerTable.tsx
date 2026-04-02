import { memo } from "react";
import { useTheme } from "../../theme";
import { useTableSort } from "../../hooks/useTableSort";
import { useTableGrouping } from "../../hooks/useTableGrouping";
import { networkColor } from "../../utils/colors";
import { SortableHeader } from "./SortableHeader";
import { TableGroupHeader } from "./TableGroupHeader";
import { ContainerRow } from "./ContainerRow";
import { TableToolbar } from "./TableToolbar";
import { CONTAINER_GRID, tableLayout } from "./tableStyles";
import type { DGNode } from "../../types";
import type { ContainerStatsData } from "../../types/stats";

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
  const { theme } = useTheme();
  const sort = useTableSort<string>("name");
  const grouping = useTableGrouping(nodes);

  const layout = tableLayout(theme);

  const isGrouped = grouping.groupBy !== "none";

  return (
    <>
      <TableToolbar groupBy={grouping.groupBy} onGroupByChange={grouping.setGroupBy} />
      <div style={{ ...layout.scrollBody, display: "flex", flexDirection: "column", gap: 12 }}>
        {grouping.groups.map((group) => {
          const sorted = sort.sortItems(group.items, (n) => ({
            name: n.name,
            image: n.image ?? "",
            status: n.status ?? "",
            ports: n.ports?.map((p) => `${p.host}:${p.container}`).join(",") ?? "",
            network: n.networkId?.replace(/^network:/, "") ?? "",
          }));
          const collapsed = grouping.isCollapsed(group.key);
          const color = group.key.startsWith("network:") ? networkColor(group.label) : undefined;

          return (
            <div key={group.key} style={layout.card}>
              {isGrouped && (
                <TableGroupHeader
                  label={group.label}
                  count={group.items.length}
                  collapsed={collapsed}
                  onToggle={() => grouping.toggleCollapsed(group.key)}
                  color={color}
                />
              )}
              {!collapsed && (
                <>
                  <SortableHeader
                    columns={COLUMNS}
                    sortColumn={sort.column}
                    sortDirection={sort.direction}
                    onSort={sort.toggleSort}
                    gridTemplate={CONTAINER_GRID}
                  />
                  {sorted.map((node) => (
                    <ContainerRow
                      key={node.id}
                      node={node}
                      stats={statsMap.get(node.name)}
                      selected={node.id === selectedNodeId}
                      onClick={onRowClick}
                      gridTemplate={CONTAINER_GRID}
                    />
                  ))}
                </>
              )}
            </div>
          );
        })}
        {nodes.length === 0 && (
          <div style={{ padding: 32, textAlign: "center", color: theme.nodeSubtext, fontSize: 13 }}>
            No containers found
          </div>
        )}
      </div>
    </>
  );
});
