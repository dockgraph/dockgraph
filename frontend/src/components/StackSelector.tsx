import { memo, useEffect, useId, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from "react";
import { useTheme } from "../theme";
import { STANDALONE_STACK, type StackSummary } from "../utils/stack";

interface Props {
  /** Every stack in the (unscoped) graph, from listStacks. */
  stacks: StackSummary[];
  /** Selected stack, or null for all. */
  selected: string | null;
  onSelect: (stack: string | null) => void;
}

interface Option {
  key: string | null;
  label: string;
  running: number;
  total: number;
}

function stackLabel(name: string): string {
  return name === STANDALONE_STACK ? "Standalone" : name;
}

/**
 * Header dropdown that scopes the graph, table, dashboard and logs to one
 * compose project / swarm stack. Each entry shows its running/total workloads.
 * Hidden when there is nothing to choose between.
 *
 * Keyboard: follows the WAI-ARIA listbox pattern. Arrow Down/Up (or Enter,
 * Space) on the trigger open the list with focus on it; Arrow keys, Home and
 * End move the highlighted option (aria-activedescendant); Enter or Space
 * selects it; Escape closes and returns focus to the trigger; Tab closes.
 */
export const StackSelector = memo(function StackSelector({ stacks, selected, onSelect }: Props) {
  const { theme } = useTheme();
  const [open, setOpen] = useState(false);
  // Index of the highlighted option (keyboard focus or mouse hover).
  const [activeIndex, setActiveIndex] = useState(0);
  const ref = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const idPrefix = useId();
  const optionId = (i: number) => `${idPrefix}-opt-${i}`;

  useEffect(() => {
    if (!open) return;
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  // Move focus into the list when it opens, so arrow keys work at once.
  useEffect(() => {
    if (open) listRef.current?.focus();
  }, [open]);

  // Keep the highlighted option in view while navigating a long list.
  useEffect(() => {
    if (!open) return;
    const el = document.getElementById(`${idPrefix}-opt-${activeIndex}`);
    el?.scrollIntoView?.({ block: "nearest" });
  }, [open, activeIndex, idPrefix]);

  const hasNamedStack = stacks.some((s) => s.name !== STANDALONE_STACK);
  if (!hasNamedStack && !selected) return null;

  const all: Option = {
    key: null,
    label: "All stacks",
    running: stacks.reduce((n, s) => n + s.running, 0),
    total: stacks.reduce((n, s) => n + s.total, 0),
  };
  const options: Option[] = [
    all,
    ...stacks.map((s) => ({ key: s.name, label: stackLabel(s.name), running: s.running, total: s.total })),
  ];
  // Keep a URL-selected stack visible even before (or after) it has workloads.
  if (selected && !stacks.some((s) => s.name === selected)) {
    options.push({ key: selected, label: stackLabel(selected), running: 0, total: 0 });
  }
  const active = options.find((o) => o.key === selected) ?? all;
  const selectedIndex = Math.max(0, options.findIndex((o) => o.key === selected));

  const openList = () => {
    setActiveIndex(selectedIndex);
    setOpen(true);
  };
  const close = (refocus: boolean) => {
    setOpen(false);
    if (refocus) triggerRef.current?.focus();
  };
  const choose = (index: number) => {
    onSelect(options[index].key);
    close(true);
  };

  const handleTriggerKey = (e: ReactKeyboardEvent<HTMLButtonElement>) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      openList();
    }
  };

  const handleListKey = (e: ReactKeyboardEvent<HTMLDivElement>) => {
    const last = options.length - 1;
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setActiveIndex((i) => Math.min(last, i + 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        setActiveIndex((i) => Math.max(0, i - 1));
        break;
      case "Home":
        e.preventDefault();
        setActiveIndex(0);
        break;
      case "End":
        e.preventDefault();
        setActiveIndex(last);
        break;
      case "Enter":
      case " ":
        e.preventDefault();
        choose(Math.min(activeIndex, last));
        break;
      case "Escape":
        e.preventDefault();
        close(true);
        break;
      case "Tab":
        close(false);
        break;
    }
  };

  return (
    <div ref={ref} style={{ position: "relative", flex: "0 1 auto", minWidth: 0 }}>
      <button
        ref={triggerRef}
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label="Stack"
        title="Scope the view to one stack or project"
        onClick={() => (open ? close(false) : openList())}
        onKeyDown={handleTriggerKey}
        style={{
          display: "flex",
          alignItems: "center",
          gap: 6,
          maxWidth: 220,
          background: selected ? theme.rowHover : theme.canvasBg,
          color: selected ? theme.nodeText : theme.panelText,
          border: `1px solid ${selected ? theme.accent : theme.panelBorder}`,
          borderRadius: 8,
          padding: "5px 10px",
          fontSize: 12,
          cursor: "pointer",
        }}
      >
        <span style={{ color: theme.nodeSubtext, fontSize: 11 }}>Stack</span>
        <span style={{ fontWeight: 600, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {active.label}
        </span>
        <span aria-hidden="true" style={{ color: theme.nodeSubtext, fontSize: 9 }}>{open ? "▲" : "▼"}</span>
      </button>
      {open && (
        <div
          ref={listRef}
          role="listbox"
          aria-label="Stacks"
          tabIndex={-1}
          aria-activedescendant={optionId(activeIndex)}
          onKeyDown={handleListKey}
          style={{
            position: "absolute",
            top: "calc(100% + 4px)",
            left: 0,
            minWidth: 220,
            maxHeight: 360,
            overflowY: "auto",
            background: theme.panelBg,
            border: `1px solid ${theme.panelBorder}`,
            borderRadius: 6,
            zIndex: 20,
            outline: "none",
            boxShadow: theme.mode === "dark" ? "0 4px 12px rgba(0,0,0,0.4)" : "0 4px 12px rgba(0,0,0,0.1)",
          }}
        >
          {options.map((opt, i) => {
            const isActive = opt.key === selected;
            const isHighlighted = i === activeIndex;
            let bg = "transparent";
            if (isActive) bg = theme.panelBorder;
            else if (isHighlighted) bg = theme.rowHover;
            return (
              <div
                key={opt.key ?? "__all"}
                id={optionId(i)}
                role="option"
                aria-selected={isActive}
                onClick={() => choose(i)}
                onMouseEnter={() => setActiveIndex(i)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  gap: 12,
                  padding: "6px 12px",
                  fontSize: 12,
                  fontWeight: isActive ? 600 : 400,
                  cursor: "pointer",
                  color: isActive ? theme.accent : theme.panelText,
                  background: bg,
                  boxShadow: isHighlighted ? `inset 2px 0 0 ${theme.accent}` : "none",
                  whiteSpace: "nowrap",
                  fontStyle: opt.key === STANDALONE_STACK ? "italic" : "normal",
                }}
              >
                <span style={{ overflow: "hidden", textOverflow: "ellipsis" }}>{opt.label}</span>
                <span style={{ fontFamily: "var(--dg-font-mono)", fontSize: 11, color: theme.nodeSubtext, fontStyle: "normal" }}>
                  {opt.running}/{opt.total}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
});
