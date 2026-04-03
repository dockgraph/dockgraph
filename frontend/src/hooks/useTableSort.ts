import { useState, useCallback } from "react";

type Direction = "asc" | "desc";

export interface TableSortState<K extends string = string> {
  column: K;
  direction: Direction;
  toggleSort: (col: K) => void;
  sortItems: <T>(items: T[], keyFn: (item: T) => Record<K, string | number | undefined>) => T[];
}

export function useTableSort<K extends string = string>(defaultColumn: K): TableSortState<K> {
  const [column, setColumn] = useState<K>(defaultColumn);
  const [direction, setDirection] = useState<Direction>("asc");

  const toggleSort = useCallback(
    (col: K) => {
      setColumn((prev) => {
        if (prev === col) {
          setDirection((d) => (d === "asc" ? "desc" : "asc"));
          return prev;
        }
        setDirection("asc");
        return col;
      });
    },
    [],
  );

  const sortItems = useCallback(
    <T>(items: T[], keyFn: (item: T) => Record<K, string | number | undefined>): T[] => {
      const sorted = [...items];
      sorted.sort((a, b) => {
        const aVal = keyFn(a)[column];
        const bVal = keyFn(b)[column];
        if (aVal == null && bVal == null) return 0;
        if (aVal == null) return 1;
        if (bVal == null) return -1;
        const cmp = typeof aVal === "number" && typeof bVal === "number"
          ? aVal - bVal
          : String(aVal).localeCompare(String(bVal));
        return direction === "asc" ? cmp : -cmp;
      });
      return sorted;
    },
    [column, direction],
  );

  return { column, direction, toggleSort, sortItems };
}
