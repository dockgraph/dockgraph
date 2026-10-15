import { useState } from "react";
import { useTheme } from "../theme";
import type { GroupBy } from "../hooks/useGroupBy";

interface SegmentOption<T extends string> {
  key: T;
  label: string;
}

interface SegmentedToggleProps<T extends string> {
  /** Accessible name of the control, also shown as a leading caption. */
  label: string;
  options: readonly SegmentOption<T>[];
  value: T;
  onChange: (value: T) => void;
  disabled?: boolean;
  /** Tooltip, e.g. why the control is disabled. */
  title?: string;
}

/**
 * Small segmented control in the style of the view tabs and time range
 * selector: a recessed track with the active option raised.
 */
export function SegmentedToggle<T extends string>({ label, options, value, onChange, disabled = false, title }: SegmentedToggleProps<T>) {
  const { theme } = useTheme();
  const [hovered, setHovered] = useState<T | null>(null);

  return (
    <div
      role="group"
      aria-label={label}
      title={title}
      style={{
        display: "flex",
        alignItems: "center",
        flex: "0 0 auto",
        background: theme.canvasBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 8,
        padding: 3,
        gap: 2,
        opacity: disabled ? 0.5 : 1,
      }}
    >
      <span style={{ color: theme.nodeSubtext, fontSize: 11, padding: "0 4px 0 6px" }}>{label}</span>
      {options.map(({ key, label: optLabel }) => {
        const active = key === value;
        const hot = !active && !disabled && hovered === key;
        return (
          <button
            key={key}
            type="button"
            aria-pressed={active}
            disabled={disabled}
            onClick={active || disabled ? undefined : () => onChange(key)}
            onMouseEnter={() => setHovered(key)}
            onMouseLeave={() => setHovered((h) => (h === key ? null : h))}
            style={{
              padding: "4px 10px",
              borderRadius: 6,
              border: "none",
              fontFamily: "var(--dg-font-ui)",
              fontSize: 11.5,
              fontWeight: active ? 600 : 500,
              cursor: active || disabled ? "default" : "pointer",
              background: active ? theme.nodeBg : hot ? theme.rowHover : "transparent",
              color: active ? theme.accent : hot ? theme.nodeText : theme.nodeSubtext,
              boxShadow: active ? `inset 0 0 0 1px ${theme.accentSoft}, 0 1px 2px rgba(0, 0, 0, 0.25)` : "none",
              transition: "background 0.15s, color 0.15s",
              lineHeight: 1.2,
            }}
          >
            {optLabel}
          </button>
        );
      })}
    </div>
  );
}

const GROUP_OPTIONS = [
  { key: "network", label: "Network" },
  { key: "node", label: "Node" },
] as const;

interface Props {
  value: GroupBy;
  onChange: (value: GroupBy) => void;
}

/** Graph header switch between network grouping and the per-swarm-node view. */
export function GroupByToggle({ value, onChange }: Props) {
  return (
    <SegmentedToggle
      label="Group"
      options={GROUP_OPTIONS}
      value={value}
      onChange={onChange}
      title="Group the graph by network or by swarm node"
    />
  );
}
