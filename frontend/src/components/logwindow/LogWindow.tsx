import { useCallback, useEffect, useRef } from 'react';
import { useTheme } from '../../theme';
import { useContainerLogs } from '../../hooks/useContainerLogs';
import { useDragResize, suppressSelection, type Rect, type ResizeEdge } from '../../hooks/useDragResize';
import { WINDOW_MIN_W, WINDOW_MIN_H } from '../../utils/constants';
import type { LogWindowState } from '../../hooks/logWindowsState';
import { LogStream } from './LogStream';

export interface LogWindowHandlers {
  onFocus: (id: string) => void;
  onClose: (id: string) => void;
  onCloseTab: (id: string, tabIndex: number) => void;
  onMinimize: (id: string) => void;
  onMove: (id: string, x: number, y: number) => void;
  onResize: (id: string, w: number, h: number) => void;
  onSetActiveTab: (id: string, tabIndex: number) => void;
  onSetSearch: (id: string, tabIndex: number, q: string) => void;
  /** Open the side info panel for a container. */
  onTitleClick: (containerId: string) => void;
  /** Live pointer position during a title-bar drag (for merge hit-testing). */
  onDragMove: (id: string, clientX: number, clientY: number) => void;
  /** Drag finished — the layer resolves a pending merge here. */
  onDragEnd: (id: string) => void;
  /** A tab was dragged out — detach it into a new window at (x, y). */
  onTabDetach: (id: string, tabIndex: number, x: number, y: number) => void;
}

interface Props extends LogWindowHandlers {
  win: LogWindowState;
  flashed: boolean;
}

const EDGES: ResizeEdge[] = ['n', 's', 'e', 'w', 'ne', 'nw', 'se', 'sw'];

/** Distance (px) a tab must move before it detaches into its own window. */
const DETACH_THRESHOLD = 28;

/** Offset (px) placing a detached window so the pointer lands on its title bar. */
const DETACH_GRAB_OFFSET = { x: 40, y: 10 };

export function LogWindow(props: Props) {
  const { win, flashed } = props;
  const { theme } = useTheme();
  // Mirror the live geometry into a ref so drag/resize gestures read the
  // current rect at gesture start without re-subscribing pointer handlers.
  const rectRef = useRef<Rect>({ x: win.x, y: win.y, w: win.w, h: win.h });
  useEffect(() => {
    rectRef.current = { x: win.x, y: win.y, w: win.w, h: win.h };
  }, [win.x, win.y, win.w, win.h]);

  const getRect = useCallback(() => rectRef.current, []);
  const { startMove, startResize } = useDragResize(getRect, { w: WINDOW_MIN_W, h: WINDOW_MIN_H }, {
    onMove: (x, y) => props.onMove(win.id, x, y),
    onResize: (r) => {
      props.onMove(win.id, r.x, r.y);
      props.onResize(win.id, r.w, r.h);
    },
    onDrag: (cx, cy) => props.onDragMove(win.id, cx, cy),
    onEnd: () => props.onDragEnd(win.id),
  });

  const tab = win.tabs[win.activeTab];
  const multiTab = win.tabs.length > 1;

  // Tab pointer-down: a small move selects the tab; a larger drag detaches it.
  const startTabDrag = useCallback(
    (tabIndex: number) => (e: React.PointerEvent) => {
      e.stopPropagation();
      const startX = e.clientX;
      const startY = e.clientY;
      let detached = false;
      const restore = suppressSelection();

      const onPointerMove = (ev: PointerEvent) => {
        if (detached) return;
        const dist = Math.hypot(ev.clientX - startX, ev.clientY - startY);
        if (dist > DETACH_THRESHOLD && win.tabs.length > 1) {
          detached = true;
          props.onTabDetach(win.id, tabIndex, ev.clientX - DETACH_GRAB_OFFSET.x, ev.clientY - DETACH_GRAB_OFFSET.y);
        }
      };
      const onPointerUp = () => {
        window.removeEventListener('pointermove', onPointerMove);
        window.removeEventListener('pointerup', onPointerUp);
        restore();
        if (!detached) props.onSetActiveTab(win.id, tabIndex);
      };
      window.addEventListener('pointermove', onPointerMove);
      window.addEventListener('pointerup', onPointerUp);
    },
    [win.id, win.tabs.length, props],
  );

  return (
    <div
      onPointerDown={() => props.onFocus(win.id)}
      style={{
        position: 'fixed',
        left: win.x,
        top: win.y,
        width: win.w,
        height: win.h,
        background: theme.windowBg,
        border: `1px solid ${flashed ? theme.dropIndicator : theme.windowBorder}`,
        borderRadius: 8,
        boxShadow: theme.windowShadow,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
        pointerEvents: 'auto',
        transition: flashed ? 'border-color 0.2s' : undefined,
      }}
    >
      {/* Title bar (drag handle) */}
      <div
        onPointerDown={startMove}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          padding: '5px 8px',
          background: theme.panelBg,
          borderBottom: `1px solid ${theme.windowBorder}`,
          cursor: 'move',
          userSelect: 'none',
          flex: '0 0 auto',
        }}
      >
        <button
          type="button"
          onPointerDown={(e) => e.stopPropagation()}
          onClick={() => props.onTitleClick(tab.containerId)}
          title="Open info panel"
          style={{
            background: 'transparent',
            border: 'none',
            color: theme.nodeText,
            fontSize: 12,
            fontWeight: 600,
            cursor: 'pointer',
            padding: 0,
            fontFamily: 'var(--dg-font-mono)',
            maxWidth: '60%',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {tab.title}
        </button>
        <span style={{ marginLeft: 'auto', display: 'flex', gap: 4 }}>
          <button
            type="button"
            aria-label="Minimize window"
            onPointerDown={(e) => e.stopPropagation()}
            onClick={() => props.onMinimize(win.id)}
            style={ctrlBtn(theme)}
          >
            –
          </button>
          <button
            type="button"
            aria-label="Close window"
            onPointerDown={(e) => e.stopPropagation()}
            onClick={() => props.onClose(win.id)}
            style={ctrlBtn(theme)}
          >
            ✕
          </button>
        </span>
      </div>

      {/* Tab bar (only when merged) */}
      {multiTab && (
        <div style={{ display: 'flex', gap: 2, padding: '4px 6px 0', background: theme.panelBg, flex: '0 0 auto', overflowX: 'auto' }}>
          {win.tabs.map((t, i) => (
            <div
              key={t.containerId}
              onPointerDown={startTabDrag(i)}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 4,
                cursor: 'pointer',
                padding: '3px 8px',
                borderRadius: '4px 4px 0 0',
                fontSize: 11,
                fontFamily: 'var(--dg-font-mono)',
                background: i === win.activeTab ? theme.tabActiveBg : 'transparent',
                color: i === win.activeTab ? theme.nodeText : theme.panelText,
                whiteSpace: 'nowrap',
              }}
            >
              {t.title}
              <button
                type="button"
                aria-label={`Close tab ${t.title}`}
                onPointerDown={(e) => e.stopPropagation()}
                onClick={(e) => {
                  e.stopPropagation();
                  props.onCloseTab(win.id, i);
                }}
                style={{ background: 'transparent', border: 'none', color: 'inherit', cursor: 'pointer', padding: 0, fontSize: 10 }}
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Body: active tab logs. key forces a fresh stream when the active tab changes. */}
      <div style={{ flex: 1, minHeight: 0, padding: 8 }}>
        <LogStreamForTab
          key={tab.containerId}
          containerId={tab.containerId}
          search={tab.search}
          onSetSearch={(q) => props.onSetSearch(win.id, win.activeTab, q)}
        />
      </div>

      {/* Resize handles */}
      {EDGES.map((edge) => (
        <div key={edge} onPointerDown={startResize(edge)} style={resizeHandleStyle(edge)} />
      ))}
    </div>
  );
}

/** Active-tab body wired to the per-tab search setter. Isolated so only the mounted tab streams. */
function LogStreamForTab({
  containerId,
  search,
  onSetSearch,
}: {
  containerId: string;
  search: string;
  onSetSearch: (q: string) => void;
}) {
  const logs = useContainerLogs(containerId, true);
  return <LogStream {...logs} active showSearch search={search} onSearchChange={onSetSearch} />;
}

function ctrlBtn(theme: { panelBorder: string; panelText: string }): React.CSSProperties {
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
    fontSize: 11,
    padding: 0,
  };
}

function resizeHandleStyle(edge: ResizeEdge): React.CSSProperties {
  const thick = 6;
  const map: Record<ResizeEdge, React.CSSProperties> = {
    n: { top: 0, left: thick, right: thick, height: thick, cursor: 'ns-resize' },
    s: { bottom: 0, left: thick, right: thick, height: thick, cursor: 'ns-resize' },
    e: { top: thick, bottom: thick, right: 0, width: thick, cursor: 'ew-resize' },
    w: { top: thick, bottom: thick, left: 0, width: thick, cursor: 'ew-resize' },
    ne: { top: 0, right: 0, width: thick, height: thick, cursor: 'nesw-resize' },
    nw: { top: 0, left: 0, width: thick, height: thick, cursor: 'nwse-resize' },
    se: { bottom: 0, right: 0, width: thick, height: thick, cursor: 'nwse-resize' },
    sw: { bottom: 0, left: 0, width: thick, height: thick, cursor: 'nesw-resize' },
  };
  return { position: 'absolute', zIndex: 2, ...map[edge] };
}
