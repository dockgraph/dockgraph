import { useEffect, useMemo, useState, useCallback } from 'react';
import { useTheme } from '../../theme';
import { useAggregateLogs } from '../../hooks/useAggregateLogs';
import { useLogFilters, matchesFilters, type Chip } from '../../hooks/useLogFilters';
import { LogStream } from '../logwindow/LogStream';
import { LogsToolbar } from './LogsToolbar';
import { LogRowMenu } from './LogRowMenu';
import { STANDALONE_STACK } from '../../utils/stack';
import type { LogLine } from '../../types/stats';

interface Props {
  active: boolean;
  /** Open a container's side info panel. */
  onOpenContainer: (container: string) => void;
  /** Stack scope (null = all). Named stacks are filtered server-side. */
  stack?: string | null;
  /**
   * Workload names in scope. Used only for the standalone scope, which the
   * backend can't express, to drop lines from project-owned workloads.
   */
  scopeNames?: Set<string>;
}

export function CommonLogs({ active, onOpenContainer, stack = null, scopeNames }: Props) {
  const { theme } = useTheme();
  const standalone = stack === STANDALONE_STACK;
  const logs = useAggregateLogs(active, standalone ? null : stack);
  const { filters, setText, addChip, removeChip, toggleRegex, clear } = useLogFilters();
  const [paused, setPaused] = useState(false);
  const [menu, setMenu] = useState<{ line: LogLine; anchor: DOMRect } | null>(null);

  // Ctrl+F opens an in-view find bar (highlight + cycle) over the shown lines,
  // distinct from the toolbar filter which hides non-matching lines.
  const [findOpen, setFindOpen] = useState(false);
  const [findText, setFindText] = useState('');
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'f') {
        e.preventDefault();
        setFindOpen((o) => !o);
      } else if (e.key === 'Escape') {
        setFindOpen(false);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  // Freeze the line list while paused so the user can read without churn.
  const [frozen, setFrozen] = useState<LogLine[] | null>(null);
  const sourceLines = paused && frozen ? frozen : logs.lines;

  const scopedLines = useMemo(
    () => (standalone && scopeNames ? sourceLines.filter((l) => !l.container || scopeNames.has(l.container)) : sourceLines),
    [sourceLines, standalone, scopeNames],
  );
  const shown = useMemo(() => scopedLines.filter((l) => matchesFilters(l, filters)), [scopedLines, filters]);

  const togglePause = useCallback(() => {
    setPaused((p) => {
      const next = !p;
      setFrozen(next ? logs.lines : null);
      return next;
    });
  }, [logs.lines]);

  const onPick = useCallback((chip: Chip) => addChip(chip), [addChip]);
  const onCopy = useCallback((line: LogLine) => {
    void navigator.clipboard?.writeText(`${line.container ?? ''} ${line.timestamp ?? ''} ${line.text}`.trim());
  }, []);

  return (
    <div
      style={{
        position: 'absolute', inset: 0, top: 50, display: 'flex', flexDirection: 'column',
        background: theme.canvasBg,
      }}
    >
      <LogsToolbar
        filters={filters}
        onText={setText}
        onRemoveChip={removeChip}
        onClear={clear}
        onToggleRegex={toggleRegex}
        paused={paused}
        onTogglePause={togglePause}
        shown={shown.length}
        total={scopedLines.length}
      />
      <div style={{ flex: 1, minHeight: 0, padding: 8 }}>
        <LogStream
          lines={shown}
          connected={logs.connected}
          loading={logs.loading}
          loadingMore={logs.loadingMore}
          hasMore={logs.hasMore}
          loadMore={logs.loadMore}
          active={active}
          showContainer
          onContainerClick={onOpenContainer}
          onLineMenu={(line, anchor) => setMenu({ line, anchor })}
          showSearch={findOpen}
          search={findText}
          onSearchChange={setFindText}
        />
      </div>
      {menu && (
        <LogRowMenu
          line={menu.line}
          anchor={menu.anchor}
          onPick={onPick}
          onCopy={onCopy}
          onClose={() => setMenu(null)}
        />
      )}
    </div>
  );
}
