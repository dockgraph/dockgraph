import { useState, useMemo, useCallback } from "react";
import type { DGNode } from "../types";

export type GroupByKey = "network" | "compose" | "status" | "driver" | "none";

export interface Group<T> {
  key: string;
  label: string;
  items: T[];
}

export interface TableGroupingState<T> {
  groupBy: GroupByKey;
  setGroupBy: (key: GroupByKey) => void;
  groups: Group<T>[];
  isCollapsed: (key: string) => boolean;
  toggleCollapsed: (key: string) => void;
}

function formatGroupLabel(key: string, groupBy: GroupByKey): string {
  if (key === "_unassigned") return "Unassigned";
  if (key === "_no_project") return "No project";
  if (key === "_no_driver") return "No driver";
  if (key === "_all") return "All";
  if (groupBy === "network") return key.replace(/^network:/, "");
  return key.charAt(0).toUpperCase() + key.slice(1);
}

function nodeGroupKey(node: DGNode, groupBy: GroupByKey): string {
  switch (groupBy) {
    case "network":
      return node.networkId ?? "_unassigned";
    case "compose":
      return node.labels?.["com.docker.compose.project"] ?? "_no_project";
    case "status":
      return node.status ?? "unknown";
    case "driver":
      return node.driver ?? "_no_driver";
    case "none":
      return "_all";
  }
}

function groupNodes(nodes: DGNode[], groupBy: GroupByKey): Group<DGNode>[] {
  if (groupBy === "none") {
    return [{ key: "_all", label: "All", items: nodes }];
  }

  const map = new Map<string, DGNode[]>();

  for (const node of nodes) {
    const key = nodeGroupKey(node, groupBy);
    const list = map.get(key);
    if (list) {
      list.push(node);
    } else {
      map.set(key, [node]);
    }
  }

  return Array.from(map.entries()).map(([key, items]) => ({
    key,
    label: formatGroupLabel(key, groupBy),
    items,
  }));
}

export function useTableGrouping(
  nodes: DGNode[],
  defaultGroupBy: GroupByKey = "compose",
): TableGroupingState<DGNode> {
  const [groupBy, setGroupBy] = useState<GroupByKey>(defaultGroupBy);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  const groups = useMemo(() => groupNodes(nodes, groupBy), [nodes, groupBy]);

  const isCollapsed = useCallback((key: string) => collapsed.has(key), [collapsed]);

  const toggleCollapsed = useCallback((key: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }, []);

  return { groupBy, setGroupBy, groups, isCollapsed, toggleCollapsed };
}
