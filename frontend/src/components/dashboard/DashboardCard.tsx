import { type ReactNode, memo } from "react";
import { useTheme, type Theme } from "../../theme";

interface Props {
  title: string;
  children: ReactNode;
}

const cardStyles = (theme: Theme) => ({
  container: {
    background: theme.cardBg,
    border: `1px solid ${theme.panelBorder}`,
    borderRadius: 8,
    padding: 16,
    display: "flex",
    flexDirection: "column" as const,
    gap: 12,
    minWidth: 0,
  },
  title: {
    fontSize: 12,
    fontWeight: 600,
    color: theme.nodeSubtext,
    textTransform: "uppercase" as const,
    letterSpacing: "0.05em",
    margin: 0,
  },
});

export const DashboardCard = memo(function DashboardCard({ title, children }: Props) {
  const { theme } = useTheme();
  const styles = cardStyles(theme);

  return (
    <div style={styles.container}>
      <p style={styles.title}>{title}</p>
      {children}
    </div>
  );
});
