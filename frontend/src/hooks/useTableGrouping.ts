import { useState, useMemo, useCallback } from "react";
import type { DGNode } from "../types";

export type GroupByKey = "network" | "compose" | "status" | "none";

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
  if (key === "_all") return "All";
  if (groupBy === "network") return key.replace(/^network:/, "");
  return key.charAt(0).toUpperCase() + key.slice(1);
}

function groupNodes(nodes: DGNode[], groupBy: GroupByKey): Group<DGNode>[] {
  if (groupBy === "none") {
    return [{ key: "_all", label: "All", items: nodes }];
  }

  const map = new Map<string, DGNode[]>();

  for (const node of nodes) {
    let key: string;
    switch (groupBy) {
      case "network":
        key = node.networkId ?? "_unassigned";
        break;
      case "compose":
        key = node.labels?.["com.docker.compose.project"] ?? "_no_project";
        break;
      case "status":
        key = node.status ?? "unknown";
        break;
    }
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

export function useTableGrouping(nodes: DGNode[]): TableGroupingState<DGNode> {
  const [groupBy, setGroupBy] = useState<GroupByKey>("compose");
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
