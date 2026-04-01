import { useRef, useEffect } from 'react';
import { useTheme } from '../theme';
import type { SearchFilterResult } from '../hooks/useSearchFilter';
import { SearchFilterChips } from './SearchFilterChips';

interface Props {
  search: SearchFilterResult;
}

export function SearchFilter({ search }: Props) {
  const { theme } = useTheme();
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === '/' && document.activeElement?.tagName !== 'INPUT') {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, []);

  return (
    <div
      style={{
        position: 'absolute',
        top: 10,
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 10,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 4,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          background: theme.panelBg,
          border: `1px solid ${theme.panelBorder}`,
          borderRadius: 6,
          padding: '4px 10px',
        }}
      >
        <span style={{ fontSize: 12, color: theme.nodeSubtext }}>&#x2315;</span>
        <input
          ref={inputRef}
          type="text"
          value={search.query}
          onChange={(e) => search.setQuery(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Escape') { search.clearAll(); inputRef.current?.blur(); } }}
          placeholder="Search containers, images\u2026"
          style={{
            background: 'transparent',
            border: 'none',
            outline: 'none',
            color: theme.panelText,
            fontSize: 12,
            width: 200,
          }}
        />
        {search.hasActiveFilter && (
          <>
            <span style={{ fontSize: 10, color: theme.nodeSubtext }}>{search.matchCount}/{search.totalCount}</span>
            <button
              onClick={search.clearAll}
              style={{ background: 'none', border: 'none', color: theme.nodeSubtext, fontSize: 12, cursor: 'pointer', padding: 0 }}
              aria-label="Clear search"
            >
              &#x2715;
            </button>
          </>
        )}
      </div>
      <SearchFilterChips search={search} />
    </div>
  );
}
