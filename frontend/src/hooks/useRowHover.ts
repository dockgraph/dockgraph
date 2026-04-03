import { useState, useCallback } from "react";
import type { Theme } from "../theme";

interface RowHoverResult {
  hovered: boolean;
  handlers: {
    onMouseEnter: () => void;
    onMouseLeave: () => void;
  };
  rowStyle: (theme: Theme, selected: boolean, base?: React.CSSProperties) => React.CSSProperties;
}

export function useRowHover(): RowHoverResult {
  const [hovered, setHovered] = useState(false);

  const handlers = {
    onMouseEnter: useCallback(() => setHovered(true), []),
    onMouseLeave: useCallback(() => setHovered(false), []),
  };

  const rowStyle = useCallback(
    (theme: Theme, selected: boolean, base?: React.CSSProperties): React.CSSProperties => ({
      ...base,
      ...(hovered ? { background: theme.rowHover } : {}),
      ...(selected ? { background: theme.panelBorder } : {}),
    }),
    [hovered],
  );

  return { hovered, handlers, rowStyle };
}
