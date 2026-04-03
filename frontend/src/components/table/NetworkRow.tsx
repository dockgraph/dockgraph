import { memo } from "react";
import { useTheme } from "../../theme";
import { useRowHover } from "../../hooks/useRowHover";
import { tableRow } from "./tableStyles";
import type { DGNode } from "../../types";

interface Props {
  node: DGNode;
  containerCount: number;
  selected: boolean;
  onClick: (nodeId: string) => void;
  gridTemplate: string;
}

export const NetworkRow = memo(function NetworkRow({
  node,
  containerCount,
  selected,
  onClick,
  gridTemplate,
}: Props) {
  const { theme } = useTheme();
  const styles = tableRow(theme);
  const { handlers, rowStyle } = useRowHover();

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => onClick(node.id)}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onClick(node.id); } }}
      {...handlers}
      style={rowStyle(theme, selected, { ...styles.row, gridTemplateColumns: gridTemplate })}
    >
      <span style={{ fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={node.name}>
        {node.name}
      </span>
      <span style={{ color: theme.nodeSubtext }}>
        {node.driver ?? "\u2014"}
      </span>
      <span style={{ color: theme.nodeSubtext, fontFamily: "monospace", fontSize: 11 }}>
        {node.subnet || "\u2014"}
      </span>
      <span style={{ color: theme.nodeSubtext, fontFamily: "monospace", fontSize: 11 }}>
        {node.gateway || "\u2014"}
      </span>
      <span style={{ color: theme.nodeSubtext }}>
        {containerCount}
      </span>
    </div>
  );
});
