import { memo } from "react";
import { useTheme } from "../../theme";
import { useRowHover } from "../../hooks/useRowHover";
import { VOLUME_COLOR } from "../../utils/colors";
import { tableRow } from "./tableStyles";
import type { DGNode } from "../../types";

interface Props {
  node: DGNode;
  mountPath: string;
  containerCount: number;
  selected: boolean;
  onClick: (nodeId: string) => void;
  gridTemplate: string;
}

export const VolumeRow = memo(function VolumeRow({
  node,
  mountPath,
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
      <span style={{ display: "flex", alignItems: "center", gap: 7, overflow: "hidden" }}>
        <span style={{ width: 6, height: 6, borderRadius: "50%", background: VOLUME_COLOR, flexShrink: 0 }} />
        <span style={{ fontFamily: "var(--dg-font-mono)", fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={node.name}>
          {node.name}
        </span>
      </span>
      <span style={{ color: theme.nodeSubtext }}>
        {node.driver ?? "\u2014"}
      </span>
      <span
        title={mountPath || undefined}
        style={{
          color: theme.nodeSubtext,
          fontFamily: "var(--dg-font-mono)",
          fontSize: 11,
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap",
          direction: "rtl",
          textAlign: "left",
        }}
      >
        {mountPath || "\u2014"}
      </span>
      <span style={{ color: theme.nodeSubtext }}>
        {containerCount}
      </span>
    </div>
  );
});
