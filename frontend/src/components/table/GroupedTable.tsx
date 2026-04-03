import type { ReactNode } from "react";
import { useTheme } from "../../theme";
import { SortableHeader } from "./SortableHeader";
import { TableGroupHeader } from "./TableGroupHeader";
import { TableToolbar, type GroupOption } from "./TableToolbar";
import { tableLayout } from "./tableStyles";
import type { TableSortState } from "../../hooks/useTableSort";
import type { TableGroupingState, Group } from "../../hooks/useTableGrouping";
import type { DGNode } from "../../types";

interface Column {
  key: string;
  label: string;
}

interface Props {
  nodes: DGNode[];
  grouping: TableGroupingState<DGNode>;
  sort: TableSortState;
  columns: Column[];
  gridTemplate: string;
  groupOptions: GroupOption[];
  emptyMessage: string;
  groupColor?: (group: Group<DGNode>) => string | undefined;
  sortKeyFn: (node: DGNode) => Record<string, string | number | undefined>;
  renderRow: (node: DGNode) => ReactNode;
}

export function GroupedTable({
  nodes,
  grouping,
  sort,
  columns,
  gridTemplate,
  groupOptions,
  emptyMessage,
  groupColor,
  sortKeyFn,
  renderRow,
}: Props) {
  const { theme } = useTheme();
  const layout = tableLayout(theme);
  const isGrouped = grouping.groupBy !== "none";

  return (
    <>
      <TableToolbar groupBy={grouping.groupBy} onGroupByChange={grouping.setGroupBy} options={groupOptions} />
      <div style={{ ...layout.scrollBody, display: "flex", flexDirection: "column", gap: 12 }}>
        {grouping.groups.map((group) => {
          const sorted = sort.sortItems(group.items, sortKeyFn);
          const collapsed = grouping.isCollapsed(group.key);

          return (
            <div key={group.key} style={layout.card}>
              {isGrouped && (
                <TableGroupHeader
                  label={group.label}
                  count={group.items.length}
                  collapsed={collapsed}
                  onToggle={() => grouping.toggleCollapsed(group.key)}
                  color={groupColor?.(group)}
                />
              )}
              {!collapsed && (
                <>
                  <SortableHeader
                    columns={columns}
                    sortColumn={sort.column}
                    sortDirection={sort.direction}
                    onSort={sort.toggleSort}
                    gridTemplate={gridTemplate}
                  />
                  {sorted.map(renderRow)}
                </>
              )}
            </div>
          );
        })}
        {nodes.length === 0 && (
          <div style={{ padding: 32, textAlign: "center", color: theme.nodeSubtext, fontSize: 13 }}>
            {emptyMessage}
          </div>
        )}
      </div>
    </>
  );
}
