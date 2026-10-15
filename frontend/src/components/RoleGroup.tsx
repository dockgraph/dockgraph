import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { swarmRoleColor } from '../utils/colors';
import { useTheme } from '../theme';
import type { RoleGroupData } from '../types';

const TITLES = { manager: 'Managers', worker: 'Workers' } as const;

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? '' : 's'}`;

/**
 * "Managers" / "Workers" container of the per-node view. Shares
 * NetworkGroup's frame and legend tab, tinted with the role colour, and adds
 * a node/task summary. It is the draggable handle of its section: grabbing
 * the frame or the tab moves every box and card inside it.
 */
export const RoleGroup = memo(function RoleGroup({ data }: NodeProps) {
  const { role, nodeCount, taskCount } = data as unknown as RoleGroupData;
  const { theme } = useTheme();
  const color = swarmRoleColor(role, theme.mode);
  const title = TITLES[role];

  return (
    <div
      data-testid="role-group"
      data-role={role}
      title={`${title}: ${plural(nodeCount, 'node')}, ${plural(taskCount, 'task')} — drag to move`}
      style={{
        width: '100%',
        height: '100%',
        boxSizing: 'border-box',
        border: `1px solid ${color}${theme.groupBorderAlpha}`,
        borderRadius: 10,
        background: `${color}${theme.groupBgAlpha}`,
        position: 'relative',
        cursor: 'grab',
      }}
    >
      <NodeHandles />

      {/* Legend tab, identical to NetworkGroup's: hangs from the top border. */}
      <div
        data-testid="role-group-tab"
        style={{
          position: 'absolute',
          top: -1,
          left: 12,
          zIndex: 1,
          display: 'inline-flex',
          alignItems: 'center',
          gap: 5,
          maxWidth: 'calc(100% - 24px)',
          padding: '2px 8px',
          background: theme.canvasBg,
          border: `1px solid ${color}${theme.groupBorderAlpha}`,
          borderRadius: '0 0 6px 6px',
          fontFamily: 'var(--dg-font-mono)',
          fontSize: 10,
          lineHeight: 1.5,
          fontWeight: 600,
          color: `${color}${theme.groupTextAlpha}`,
          letterSpacing: '0.04em',
          textTransform: 'uppercase' as const,
          whiteSpace: 'nowrap',
        }}
      >
        <span
          aria-hidden="true"
          style={{ width: 6, height: 6, borderRadius: '50%', background: color, flex: '0 0 auto' }}
        />
        <span>{title}</span>
        <span
          data-testid="role-group-summary"
          style={{ color: theme.nodeSubtext, fontWeight: 500, textTransform: 'none', letterSpacing: 0 }}
        >
          {plural(nodeCount, 'node')} · {plural(taskCount, 'task')}
        </span>
      </div>
    </div>
  );
});
