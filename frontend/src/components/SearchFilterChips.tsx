import { useTheme } from '../theme';
import { STATUS_COLORS } from '../utils/colors';
import type { Theme } from '../theme';
import type { SearchFilterResult } from '../hooks/useSearchFilter';

interface Props {
  search: SearchFilterResult;
  visible?: boolean;
}

const STATUS_OPTIONS = ['running', 'exited', 'paused', 'unhealthy'] as const;
const TYPE_OPTIONS = ['container', 'volume', 'network'] as const;

interface ChipProps {
  label: string;
  active: boolean;
  onClick: () => void;
  color: string;
  showDot?: boolean;
  theme: Theme;
}

function Chip({ label, active, onClick, color, showDot, theme }: ChipProps) {
  return (
    <button
      onClick={onClick}
      aria-pressed={active}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 6,
        width: '100%',
        fontFamily: 'var(--dg-font-ui)',
        fontSize: 11,
        fontWeight: active ? 600 : 500,
        padding: '5px 10px',
        borderRadius: 7,
        border: `1px solid ${active ? color : theme.panelBorder}`,
        background: active ? `${color}22` : theme.canvasBg,
        color: active ? color : theme.nodeSubtext,
        cursor: 'pointer',
        textTransform: 'capitalize',
        transition: 'border-color 0.12s, background 0.12s, color 0.12s',
      }}
    >
      {showDot && (
        <span
          style={{
            width: 6,
            height: 6,
            borderRadius: '50%',
            background: color,
            flex: '0 0 auto',
            opacity: active ? 1 : 0.5,
          }}
        />
      )}
      {label}
    </button>
  );
}

function SectionLabel({ children, theme }: { children: string; theme: Theme }) {
  return (
    <div
      style={{
        fontFamily: 'var(--dg-font-mono)',
        fontSize: 9.5,
        fontWeight: 600,
        letterSpacing: '0.08em',
        textTransform: 'uppercase',
        color: theme.nodeSubtext,
        marginBottom: 6,
      }}
    >
      {children}
    </div>
  );
}

export function SearchFilterChips({ search, visible }: Props) {
  const { theme } = useTheme();
  if (!search.hasActiveFilter && !visible) return null;

  return (
    <div
      style={{
        width: 300,
        display: 'flex',
        flexDirection: 'column',
        gap: 12,
        background: theme.panelBg,
        border: `1px solid ${theme.panelBorder}`,
        borderRadius: 10,
        padding: 12,
        boxShadow: '0 12px 30px -16px rgba(0, 0, 0, 0.7)',
      }}
    >
      <div>
        <SectionLabel theme={theme}>Status</SectionLabel>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 6 }}>
          {STATUS_OPTIONS.map((s) => (
            <Chip
              key={s}
              label={s}
              active={search.filters.statuses.has(s)}
              onClick={() => search.toggleStatus(s)}
              color={STATUS_COLORS[s] ?? theme.accent}
              showDot
              theme={theme}
            />
          ))}
        </div>
      </div>

      <div>
        <SectionLabel theme={theme}>Type</SectionLabel>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 6 }}>
          {TYPE_OPTIONS.map((t) => (
            <Chip
              key={t}
              label={t}
              active={search.filters.types.has(t)}
              onClick={() => search.toggleType(t)}
              color={theme.accent}
              theme={theme}
            />
          ))}
        </div>
      </div>
    </div>
  );
}
