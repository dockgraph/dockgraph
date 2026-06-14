import { useTheme } from '../../theme';
import { type LogFilters, type Chip } from '../../hooks/useLogFilters';

interface Props {
  filters: LogFilters;
  onText: (text: string) => void;
  onRemoveChip: (chip: Chip) => void;
  onClear: () => void;
  onToggleRegex: () => void;
  paused: boolean;
  onTogglePause: () => void;
  shown: number;
  total: number;
}

/** Lists the active filters as removable chips. */
function activeChips(f: LogFilters): { chip: Chip; label: string }[] {
  const out: { chip: Chip; label: string }[] = [];
  if (f.include) out.push({ chip: { kind: 'include', container: f.include }, label: `only: ${f.include}` });
  for (const c of f.exclude) out.push({ chip: { kind: 'exclude', container: c }, label: `exclude: ${c}` });
  if (f.stream) out.push({ chip: { kind: 'stream', stream: f.stream }, label: `stream: ${f.stream}` });
  return out;
}

export function LogsToolbar({ filters, onText, onRemoveChip, onClear, onToggleRegex, paused, onTogglePause, shown, total }: Props) {
  const { theme } = useTheme();
  const chips = activeChips(filters);

  return (
    <div
      style={{
        display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap',
        padding: '8px 12px', borderBottom: `1px solid ${theme.panelBorder}`,
        background: theme.panelBg, fontFamily: 'var(--dg-font-ui)', fontSize: 12,
      }}
    >
      <div style={{ position: 'relative', flex: '1 1 220px', minWidth: 160, display: 'flex' }}>
        <input
          value={filters.text}
          onChange={(e) => onText(e.target.value)}
          placeholder={filters.regex ? 'Filter logs (regex)' : 'Filter logs'}
          style={{
            flex: 1, padding: '4px 36px 4px 10px', borderRadius: 6,
            border: `1px solid ${theme.panelBorder}`, background: theme.logBg, color: theme.logText,
            fontFamily: 'var(--dg-font-mono)', fontSize: 12, outline: 'none',
          }}
        />
        <button
          type="button"
          aria-label="Use regex"
          aria-pressed={filters.regex}
          title="Match using a regular expression"
          onClick={onToggleRegex}
          style={{
            position: 'absolute', right: 4, top: '50%', transform: 'translateY(-50%)',
            width: 24, height: 22, borderRadius: 4, cursor: 'pointer', fontSize: 11,
            fontFamily: 'var(--dg-font-mono)', lineHeight: 1,
            border: `1px solid ${filters.regex ? theme.accent : theme.panelBorder}`,
            background: filters.regex ? theme.accentSoft : 'transparent',
            color: filters.regex ? theme.accent : theme.panelText,
          }}
        >
          .*
        </button>
      </div>

      {chips.map(({ chip, label }) => (
        <span
          key={label}
          style={{
            display: 'inline-flex', alignItems: 'center', gap: 4, padding: '2px 8px',
            borderRadius: 12, background: theme.accentSoft, color: theme.nodeText, whiteSpace: 'nowrap',
          }}
        >
          {label}
          <button
            type="button"
            aria-label={`Remove filter ${label}`}
            onClick={() => onRemoveChip(chip)}
            style={{ background: 'transparent', border: 'none', color: 'inherit', cursor: 'pointer', padding: 0, fontSize: 11 }}
          >
            ✕
          </button>
        </span>
      ))}

      {chips.length > 0 && (
        <button type="button" onClick={onClear} style={textBtn(theme)}>clear</button>
      )}

      <span style={{ marginLeft: 'auto', color: theme.nodeSubtext, fontFamily: 'var(--dg-font-mono)' }}>
        {shown} / {total}
      </span>
      <button type="button" onClick={onTogglePause} style={textBtn(theme)}>
        {paused ? '▶ Resume' : '⏸ Pause'}
      </button>
    </div>
  );
}

function textBtn(theme: { panelBorder: string; panelText: string }): React.CSSProperties {
  return {
    background: 'transparent', border: `1px solid ${theme.panelBorder}`, color: theme.panelText,
    borderRadius: 6, padding: '3px 8px', cursor: 'pointer', fontSize: 12, whiteSpace: 'nowrap',
  };
}
