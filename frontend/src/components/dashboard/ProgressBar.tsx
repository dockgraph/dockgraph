import { memo } from "react";
import { useTheme } from "../../theme";

interface Props {
  percent: number;
  color: string;
  height?: number;
}

export const ProgressBar = memo(function ProgressBar({ percent, color, height = 3 }: Props) {
  const { theme } = useTheme();
  const radius = height / 2;

  return (
    <div style={{ height, borderRadius: radius, background: theme.panelBorder }}>
      <div style={{
        height: "100%",
        width: `${percent}%`,
        borderRadius: radius,
        background: color,
        transition: "width 0.3s ease",
      }} />
    </div>
  );
});
