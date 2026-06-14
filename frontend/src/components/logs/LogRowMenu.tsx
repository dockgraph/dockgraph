import { useEffect } from 'react';
import { useTheme } from '../../theme';
import { Z } from '../../utils/constants';
import type { LogLine } from '../../types/stats';
import type { Chip } from '../../hooks/useLogFilters';

interface Props {
  line: LogLine;
  anchor: DOMRect;
  onPick: (chip: Chip) => void;
  onCopy: (line: LogLine) => void;
  onClose: () => void;
}

export function LogRowMenu({ line, anchor, onPick, onCopy, onClose }: Props) {
  const { theme } = useTheme();

  // Close on any outside click / Escape.
  useEffect(() => {
    const close = () => onClose();
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
    window.addEventListener('pointerdown', close);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('pointerdown', close);
      window.removeEventListener('keydown', onKey);
    };
  }, [onClose]);

  const items: { label: string; run: () => void }[] = [];
  if (line.container) {
    items.push({ label: `Filter to ${line.container}`, run: () => onPick({ kind: 'include', container: line.container! }) });
    items.push({ label: `Exclude ${line.container}`, run: () => onPick({ kind: 'exclude', container: line.container! }) });
  }
  items.push({ label: `Only ${line.stream}`, run: () => onPick({ kind: 'stream', stream: line.stream }) });
  items.push({ label: 'Copy line', run: () => onCopy(line) });

  return (
    <div
      onPointerDown={(e) => e.stopPropagation()}
      style={{
        position: 'fixed',
        top: anchor.bottom + 2,
        left: anchor.left,
        zIndex: Z.header + 1,
        background: theme.windowBg,
        border: `1px solid ${theme.windowBorder}`,
        borderRadius: 6,
        boxShadow: theme.windowShadow,
        padding: 4,
        minWidth: 160,
        fontFamily: 'var(--dg-font-ui)',
        fontSize: 12,
      }}
    >
      {items.map((it) => (
        <button
          key={it.label}
          type="button"
          onClick={() => {
            it.run();
            onClose();
          }}
          style={{
            display: 'block', width: '100%', textAlign: 'left', background: 'transparent',
            border: 'none', color: theme.nodeText, cursor: 'pointer', padding: '5px 8px', borderRadius: 4,
          }}
        >
          {it.label}
        </button>
      ))}
    </div>
  );
}
