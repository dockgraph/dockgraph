import { useTheme } from '../../theme';
import { Z, DOCK_HEIGHT } from '../../utils/constants';
import type { LogWindowState } from '../../hooks/logWindowsState';

interface Props {
  minimized: LogWindowState[];
  onRestore: (id: string) => void;
}

export function LogDock({ minimized, onRestore }: Props) {
  const { theme } = useTheme();
  if (minimized.length === 0) return null;

  return (
    <div
      style={{
        position: 'fixed',
        left: 0,
        right: 0,
        bottom: 0,
        height: DOCK_HEIGHT,
        zIndex: Z.dock,
        display: 'flex',
        alignItems: 'center',
        gap: 6,
        padding: '0 8px',
        background: theme.panelBg,
        borderTop: `1px solid ${theme.panelBorder}`,
        pointerEvents: 'auto',
        overflowX: 'auto',
      }}
    >
      {minimized.map((w) => {
        const label = w.tabs[w.activeTab]?.title ?? w.tabs[0].title;
        return (
          <button
            key={w.id}
            type="button"
            onClick={() => onRestore(w.id)}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              padding: '3px 10px',
              borderRadius: 4,
              border: `1px solid ${theme.panelBorder}`,
              background: theme.windowBg,
              color: theme.nodeText,
              cursor: 'pointer',
              fontSize: 11,
              fontFamily: 'var(--dg-font-mono)',
              whiteSpace: 'nowrap',
            }}
          >
            <span style={{ width: 6, height: 6, borderRadius: 2, background: theme.accent }} />
            {label}
            {w.tabs.length > 1 && <span style={{ color: theme.panelText }}>·{w.tabs.length} tabs</span>}
          </button>
        );
      })}
    </div>
  );
}
