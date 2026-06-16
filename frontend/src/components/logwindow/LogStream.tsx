import { useEffect, useMemo, useRef, useState, useCallback } from 'react';
import type { LogLine } from '../../types/stats';
import { formatLogTimestamp } from '../../utils/logParser';
import { networkColor } from '../../utils/colors';
import { useTheme } from '../../theme';

interface Props {
  lines: LogLine[];
  connected: boolean;
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  loadMore: () => void;
  active?: boolean;
  /** Show the highlight-search bar (windows). Omit for the side panel. */
  showSearch?: boolean;
  search?: string;
  onSearchChange?: (q: string) => void;
  /** Fixed body height (px). Defaults to filling the parent via flex. */
  height?: number | string;
  /** Show a per-line color-coded container badge (aggregate view). */
  showContainer?: boolean;
  /** Click handler for a container badge. */
  onContainerClick?: (container: string) => void;
  /** Row ⋮ menu handler; receives the line and the button's bounding rect. */
  onLineMenu?: (line: LogLine, anchor: DOMRect) => void;
}

interface Segment {
  text: string;
  /** Global match index when this segment is a highlighted hit, else -1. */
  matchIndex: number;
}

/**
 * Splits each line into plain/hit segments, assigning every hit a running
 * global index so the search bar can jump between matches across all lines.
 */
function buildSegments(lines: LogLine[], query: string): { rendered: Segment[][]; total: number } {
  if (!query) return { rendered: lines.map((l) => [{ text: l.text, matchIndex: -1 }]), total: 0 };
  const q = query.toLowerCase();
  let counter = 0;
  const rendered = lines.map((line) => {
    const segs: Segment[] = [];
    const text = line.text;
    const lower = text.toLowerCase();
    let i = 0;
    while (i < text.length) {
      const idx = lower.indexOf(q, i);
      if (idx < 0) {
        segs.push({ text: text.slice(i), matchIndex: -1 });
        break;
      }
      if (idx > i) segs.push({ text: text.slice(i, idx), matchIndex: -1 });
      segs.push({ text: text.slice(idx, idx + q.length), matchIndex: counter });
      counter += 1;
      i = idx + q.length;
    }
    if (segs.length === 0) segs.push({ text, matchIndex: -1 });
    return segs;
  });
  return { rendered, total: counter };
}

export function LogStream({
  lines,
  connected,
  loading,
  loadingMore,
  hasMore,
  loadMore,
  active = true,
  showSearch = false,
  search = '',
  onSearchChange,
  height,
  showContainer = false,
  onContainerClick,
  onLineMenu,
}: Props) {
  const { theme } = useTheme();
  const scrollRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [activeMatch, setActiveMatch] = useState(0);
  const prevLineCountRef = useRef(0);
  const prevScrollHeightRef = useRef(0);

  const { rendered, total } = useMemo(() => buildSegments(lines, search), [lines, search]);

  // Clamp at render so shrinking results never point past the last match.
  const effectiveActive = total === 0 ? 0 : Math.min(activeMatch, total - 1);

  // Scroll the active match into view when navigating.
  useEffect(() => {
    if (total === 0) return;
    const el = scrollRef.current?.querySelector('[data-active-match="true"]') as HTMLElement | null;
    el?.scrollIntoView?.({ block: 'center' });
  }, [effectiveActive, total]);

  // Preserve scroll position when older lines are prepended.
  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const prevCount = prevLineCountRef.current;
    const added = lines.length - prevCount;
    if (added > 0 && prevCount > 0 && !autoScroll) {
      const newHeight = el.scrollHeight;
      if (newHeight > prevScrollHeightRef.current) {
        el.scrollTop += newHeight - prevScrollHeightRef.current;
      }
    }
    prevLineCountRef.current = lines.length;
    prevScrollHeightRef.current = el.scrollHeight;
  }, [lines, autoScroll]);

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [lines, autoScroll]);

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const { scrollTop, scrollHeight, clientHeight } = el;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 20);
    if (scrollTop < 40 && scrollHeight > clientHeight && hasMore && !loadingMore) {
      prevScrollHeightRef.current = el.scrollHeight;
      loadMore();
    }
  }, [hasMore, loadingMore, loadMore]);

  const step = useCallback(
    (delta: number) => {
      if (total === 0) return;
      setActiveMatch((m) => (m + delta + total) % total);
    },
    [total],
  );

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: height ?? '100%', minHeight: 0 }}>
      {showSearch && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
          <input
            value={search}
            onChange={(e) => {
              setActiveMatch(0);
              onSearchChange?.(e.target.value);
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') step(e.shiftKey ? -1 : 1);
            }}
            placeholder="Search logs"
            style={{
              flex: 1,
              fontFamily: 'var(--dg-font-mono)',
              fontSize: 11,
              padding: '3px 8px',
              borderRadius: 4,
              border: `1px solid ${theme.panelBorder}`,
              background: theme.logBg,
              color: theme.logText,
              outline: 'none',
            }}
          />
          {search && (
            <>
              <span style={{ fontSize: 10, color: theme.logMuted, fontFamily: 'var(--dg-font-mono)', minWidth: 34, textAlign: 'right' }}>
                {total === 0 ? '0/0' : `${effectiveActive + 1}/${total}`}
              </span>
              <button type="button" aria-label="Previous match" onClick={() => step(-1)} disabled={total === 0} style={navBtn(theme)}>
                ▲
              </button>
              <button type="button" aria-label="Next match" onClick={() => step(1)} disabled={total === 0} style={navBtn(theme)}>
                ▼
              </button>
            </>
          )}
        </div>
      )}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        style={{
          flex: 1,
          minHeight: 0,
          overflowY: 'auto',
          background: theme.logBg,
          borderRadius: 6,
          border: `1px solid ${theme.panelBorder}`,
          padding: 8,
          fontFamily: 'var(--dg-font-mono)',
          fontSize: 10,
          lineHeight: 1.5,
          position: 'relative',
        }}
      >
        {loadingMore && (
          <div style={{ textAlign: 'center', padding: '4px 0 8px', color: theme.logMuted, fontSize: 10 }}>
            Loading older logs...
          </div>
        )}
        {!loadingMore && !hasMore && lines.length > 0 && (
          <div style={{ textAlign: 'center', padding: '4px 0 8px', color: theme.logTimestamp, fontSize: 10 }}>
            Beginning of logs
          </div>
        )}
        {loading && lines.length === 0 && (
          <div style={{ color: theme.logMuted, textAlign: 'center', paddingTop: 20 }}>Loading logs...</div>
        )}
        {!loading && lines.length === 0 && (
          <div style={{ color: theme.logMuted, textAlign: 'center', paddingTop: 20 }}>
            {!active ? 'Open panel to stream logs' : connected ? 'No log output yet' : 'Connecting...'}
          </div>
        )}
        {lines.map((line, li) => (
          <div key={line.id} style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
            {showContainer && (
              <button
                type="button"
                aria-label="Row actions"
                onClick={(e) => onLineMenu?.(line, (e.currentTarget as HTMLElement).getBoundingClientRect())}
                style={{
                  background: 'transparent', border: 'none', color: theme.logMuted,
                  cursor: 'pointer', padding: '0 4px 0 0', fontSize: 11, userSelect: 'none',
                }}
              >
                ⋮
              </button>
            )}
            {showContainer && line.container && (
              <button
                type="button"
                onClick={() => onContainerClick?.(line.container!)}
                title={`Open ${line.container}`}
                style={{
                  background: 'transparent', border: 'none', cursor: 'pointer', padding: 0,
                  marginRight: 6, fontWeight: 600, color: networkColor(line.container),
                  fontFamily: 'var(--dg-font-mono)', fontSize: 10, userSelect: 'none',
                }}
              >
                {line.container}
              </button>
            )}
            {line.timestamp && (
              // Trailing space is a real word boundary so a double-click on the
              // message never merges the timestamp's last chars with the message's
              // first chars into one "word" — while both stay selectable/copyable.
              <span style={{ color: theme.logTimestamp }}>{formatLogTimestamp(line.timestamp)}{'  '}</span>
            )}
            <span style={{ color: line.stream === 'stderr' ? theme.danger : theme.logText }}>
              {rendered[li].map((seg, si) =>
                seg.matchIndex >= 0 ? (
                  <mark
                    key={si}
                    data-active-match={seg.matchIndex === effectiveActive ? 'true' : undefined}
                    style={{
                      background: seg.matchIndex === effectiveActive ? theme.dropIndicator : theme.accentSoft,
                      color: seg.matchIndex === effectiveActive ? theme.canvasBg : 'inherit',
                      borderRadius: 2,
                    }}
                  >
                    {seg.text}
                  </mark>
                ) : (
                  <span key={si}>{seg.text}</span>
                ),
              )}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function navBtn(theme: { panelBorder: string; panelText: string }): React.CSSProperties {
  return {
    width: 18,
    height: 18,
    lineHeight: '16px',
    textAlign: 'center',
    borderRadius: 4,
    border: `1px solid ${theme.panelBorder}`,
    background: 'transparent',
    color: theme.panelText,
    cursor: 'pointer',
    fontSize: 9,
    padding: 0,
  };
}
