import { useEffect, useLayoutEffect, useMemo, useRef, useState, useCallback } from 'react';
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
  /** The buffer stops short of the newest line (see useLogs); reaching the bottom catches up. */
  hasNewer?: boolean;
  loadingNewer?: boolean;
  loadNewer?: () => void;
  /** Reports whether the reader is at the bottom, following the tail. */
  onFollowChange?: (following: boolean) => void;
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

/** Distance from the end that still counts as "at the bottom", following. */
const BOTTOM_SLACK_PX = 20;

/** Pixels per line for wheel deltas reported in lines (Firefox). */
const WHEEL_LINE_PX = 16;

/**
 * Returns the first log row at least partly inside the scroll box's
 * viewport. Rows are stacked in order, so a binary search over the box's
 * children finds it without measuring every row; non-row children (the
 * "Loading older logs" banner) are skipped.
 */
function firstVisibleRow(box: HTMLElement): HTMLElement | null {
  const children = box.children;
  let lo = 0;
  let hi = children.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    const child = children[mid] as HTMLElement;
    if (child.offsetTop + child.offsetHeight <= box.scrollTop) lo = mid + 1;
    else hi = mid;
  }
  for (let i = lo; i < children.length; i++) {
    const child = children[i] as HTMLElement;
    if (child.dataset.lineId) return child;
  }
  return null;
}

export function LogStream({
  lines,
  connected,
  loading,
  loadingMore,
  hasMore,
  loadMore,
  hasNewer = false,
  loadingNewer = false,
  loadNewer,
  onFollowChange,
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
  // While the user reads older output: the row at the top of the viewport
  // and its distance from the viewport top, kept in place as lines change.
  const anchorRef = useRef<{ id: string; offset: number } | null>(null);
  // scrollTop as last seen or set here, so a scroll event can tell the user
  // scrolling up from our own adjustments.
  const lastTopRef = useRef(0);

  const { rendered, total } = useMemo(() => buildSegments(lines, search), [lines, search]);

  // Clamp at render so shrinking results never point past the last match.
  const effectiveActive = total === 0 ? 0 : Math.min(activeMatch, total - 1);

  // Scroll the active match into view when navigating.
  useEffect(() => {
    if (total === 0) return;
    const el = scrollRef.current?.querySelector('[data-active-match="true"]') as HTMLElement | null;
    el?.scrollIntoView?.({ block: 'center' });
  }, [effectiveActive, total]);

  const captureAnchor = useCallback(() => {
    const el = scrollRef.current;
    const row = el ? firstVisibleRow(el) : null;
    anchorRef.current = el && row ? { id: row.dataset.lineId!, offset: row.offsetTop - el.scrollTop } : null;
  }, []);

  // At the bottom, follow new lines. Scrolled up, keep the anchored row where
  // it was, whether lines were appended (live tail), prepended (older page)
  // or dropped from the front (buffer full). Runs before paint so the
  // content never visibly jumps.
  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    if (autoScroll) {
      el.scrollTop = el.scrollHeight;
    } else {
      const anchor = anchorRef.current;
      const row = anchor ? el.querySelector<HTMLElement>(`[data-line-id="${anchor.id}"]`) : null;
      if (anchor && row) {
        el.scrollTop = row.offsetTop - anchor.offset;
      }
      captureAnchor();
    }
    lastTopRef.current = el.scrollTop;
  }, [lines, autoScroll, captureAnchor]);

  // Starts or stops following the tail; reaching the end of a detached
  // buffer also catches up with the newer lines.
  const follow = useCallback((on: boolean) => {
    setAutoScroll(on);
    onFollowChange?.(on);
    if (on && hasNewer && !loadingNewer) loadNewer?.();
  }, [onFollowChange, hasNewer, loadingNewer, loadNewer]);

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const { scrollTop, scrollHeight, clientHeight } = el;
    const movedUp = scrollTop < lastTopRef.current;
    lastTopRef.current = scrollTop;
    const atBottom = scrollHeight - scrollTop - clientHeight < BOTTOM_SLACK_PX;
    // Follow at the bottom; stop only when the user scrolls up, so a smooth
    // scroll still on its way down never stops following.
    if (atBottom) follow(true);
    else if (movedUp) follow(false);
    if (!atBottom) captureAnchor();
    if (scrollTop < 40 && scrollHeight > clientHeight && hasMore && !loadingMore) {
      loadMore();
    }
  }, [hasMore, loadingMore, loadMore, follow, captureAnchor]);

  // A wheel scroll heading past the end resumes following at once: under fast
  // logs a smooth scroll lands short of a bottom that keeps moving, so its
  // landing position alone would never get there.
  const handleWheel = useCallback((e: React.WheelEvent<HTMLDivElement>) => {
    const el = scrollRef.current;
    if (!el || e.deltaY <= 0) return;
    let px = e.deltaY;
    if (e.deltaMode === WheelEvent.DOM_DELTA_LINE) px *= WHEEL_LINE_PX;
    else if (e.deltaMode === WheelEvent.DOM_DELTA_PAGE) px *= el.clientHeight;
    if (el.scrollTop + el.clientHeight + px >= el.scrollHeight - BOTTOM_SLACK_PX) follow(true);
  }, [follow]);

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
        data-testid="log-scroll"
        onScroll={handleScroll}
        onWheel={handleWheel}
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
          // Row offsets are measured against this box.
          position: 'relative',
          // The anchoring above keeps the viewed rows still; the browser's own
          // scroll anchoring (not in every browser) would fight it.
          overflowAnchor: 'none',
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
          <div key={line.id} data-line-id={line.id} style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
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
        {hasNewer && lines.length > 0 && (
          <div style={{ textAlign: 'center', padding: '8px 0 4px', color: theme.logMuted, fontSize: 10 }}>
            {loadingNewer ? 'Loading newer logs...' : 'Newer logs available — scroll to the bottom to catch up'}
          </div>
        )}
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
