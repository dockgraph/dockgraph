import { useRef, useEffect, useState } from "react";
import { useTheme } from "../theme";
import type { SearchFilterResult } from "../hooks/useSearchFilter";
import { SearchFilterChips } from "./SearchFilterChips";

interface Props {
  search: SearchFilterResult;
}

export function SearchFilter({ search }: Props) {
  const { theme } = useTheme();
  const inputRef = useRef<HTMLInputElement>(null);
  const [focused, setFocused] = useState(false);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "/" && document.activeElement?.tagName !== "INPUT") {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, []);

  const showHint = !focused && !search.query;

  return (
    <div
      style={{ position: "relative" }}
      onMouseDown={(e) => {
        // Prevent chip clicks from blurring the input.
        if (e.target !== inputRef.current) {
          e.preventDefault();
        }
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 7,
          // Fixed width so the field never shifts as the hint / match count
          // appear and disappear (it is center-anchored in the top bar).
          width: 320,
          // Inset relative to the top bar so the field reads as recessed.
          background: theme.canvasBg,
          border: `1px solid ${focused ? theme.accent : theme.panelBorder}`,
          borderRadius: 8,
          padding: "6px 9px 6px 10px",
          boxShadow: focused ? `0 0 0 3px ${theme.accentSoft}` : "none",
          transition: "border-color 0.15s, box-shadow 0.15s",
        }}
      >
        <span style={{ fontSize: 13, lineHeight: 1, color: focused ? theme.accent : theme.nodeSubtext }}>
          &#x2315;
        </span>
        <input
          ref={inputRef}
          type="text"
          className="dg-search-input"
          aria-label="Search containers, images, and labels"
          value={search.query}
          onChange={(e) => search.setQuery(e.target.value)}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              search.clearAll();
              inputRef.current?.blur();
            }
          }}
          placeholder="Search containers, images..."
          style={{
            flex: 1,
            minWidth: 0,
            background: "transparent",
            border: "none",
            outline: "none",
            color: theme.nodeText,
            fontFamily: "var(--dg-font-ui)",
            fontSize: 12.5,
          }}
        />
        {showHint && (
          <kbd
            aria-hidden="true"
            style={{
              flex: "0 0 auto",
              fontFamily: "var(--dg-font-mono)",
              fontSize: 10,
              lineHeight: 1.4,
              color: theme.nodeSubtext,
              background: theme.panelBg,
              border: `1px solid ${theme.panelBorder}`,
              borderRadius: 4,
              padding: "1px 5px",
            }}
          >
            /
          </kbd>
        )}
        {search.hasActiveFilter && (
          <>
            <span
              style={{
                flex: "0 0 auto",
                fontFamily: "var(--dg-font-mono)",
                fontSize: 11,
                color: theme.nodeSubtext,
                whiteSpace: "nowrap",
              }}
            >
              {search.matchCount}/{search.totalCount}
            </span>
            <button
              onClick={search.clearAll}
              style={{
                flex: "0 0 auto",
                display: "grid",
                placeItems: "center",
                background: "none",
                border: "none",
                color: theme.nodeSubtext,
                fontSize: 12,
                cursor: "pointer",
                padding: 0,
                lineHeight: 1,
              }}
              aria-label="Clear search"
            >
              &#x2715;
            </button>
          </>
        )}
      </div>
      {/* Floated as an overlay so it never resizes or shifts the field. */}
      <div
        style={{
          position: "absolute",
          top: "calc(100% + 6px)",
          left: "50%",
          transform: "translateX(-50%)",
          zIndex: 20,
        }}
      >
        <SearchFilterChips search={search} visible={focused} />
      </div>
    </div>
  );
}
