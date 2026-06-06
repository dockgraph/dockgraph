import { useTheme } from "../theme";

/**
 * Product wordmark with a small topology-graph glyph — a nod to what the
 * tool renders. Sits at the far left of the top bar.
 */
export function Brand() {
  const { theme } = useTheme();

  return (
    <div
      style={{ display: "flex", alignItems: "center", gap: 9, userSelect: "none" }}
      aria-label="DockGraph"
    >
      <span
        style={{
          width: 26,
          height: 26,
          borderRadius: 7,
          background: theme.accentSoft,
          display: "grid",
          placeItems: "center",
          flex: "0 0 auto",
        }}
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          {/* edges */}
          <path
            d="M4 4.5 8 8M12 4.5 8 8M8 8v4.5"
            stroke={theme.accent}
            strokeWidth="1.3"
            strokeLinecap="round"
            opacity="0.65"
          />
          {/* nodes */}
          <circle cx="4" cy="4.5" r="1.9" fill={theme.accent} />
          <circle cx="12" cy="4.5" r="1.9" fill={theme.accent} />
          <circle cx="8" cy="8" r="1.9" fill={theme.accent} />
          <circle cx="8" cy="12.5" r="1.9" fill={theme.accent} />
        </svg>
      </span>
      <span
        style={{
          fontFamily: "var(--dg-font-mono)",
          fontSize: 14,
          fontWeight: 600,
          letterSpacing: "-0.01em",
          color: theme.nodeText,
        }}
      >
        dock<span style={{ color: theme.accent }}>graph</span>
      </span>
    </div>
  );
}
