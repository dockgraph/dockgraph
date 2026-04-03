import { useTheme } from '../theme';
import type { SearchFilterResult } from '../hooks/useSearchFilter';

interface Props {
  search: SearchFilterResult;
  visible?: boolean;
}

const STATUS_OPTIONS = ['running', 'exited', 'paused', 'unhealthy'] as const;
const TYPE_OPTIONS = ['container', 'volume', 'network'] as const;

function Chip({ label, active, onClick, theme }: { label: string; active: boolean; onClick: () => void; theme: { panelBg: string; panelBorder: string; panelText: string } }) {
  return (
    <button
      onClick={onClick}
      style={{
        fontSize: 10,
        padding: '2px 8px',
        borderRadius: 10,
        border: `1px solid ${active ? '#3b82f6' : theme.panelBorder}`,
        background: active ? '#3b82f620' : theme.panelBg,
        color: active ? '#3b82f6' : theme.panelText,
        cursor: 'pointer',
        textTransform: 'capitalize',
      }}
    >
      {label}
    </button>
  );
}

export function SearchFilterChips({ search, visible }: Props) {
  const { theme } = useTheme();
  if (!search.hasActiveFilter && !visible) return null;

  return (
    <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap', justifyContent: 'center' }}>
      {STATUS_OPTIONS.map((s) => (
        <Chip key={s} label={s} active={search.filters.statuses.has(s)} onClick={() => search.toggleStatus(s)} theme={theme} />
      ))}
      {TYPE_OPTIONS.map((t) => (
        <Chip key={t} label={t} active={search.filters.types.has(t)} onClick={() => search.toggleType(t)} theme={theme} />
      ))}
    </div>
  );
}
