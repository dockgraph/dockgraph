import { memo } from 'react';
import type { NodeProps } from '@xyflow/react';
import { NodeHandles } from './NodeHandles';
import { InspectButton } from './InspectButton';
import { swarmRoleColor } from '../utils/colors';
import { formatCount } from '../utils/format';
import { roleTitle, roleGroupId } from '../utils/nodeTransform';
import { useTheme } from '../theme';
import type { RoleGroupData } from '../types';

/**
 * "Managers" / "Workers" container of the per-node view. Shares
 * NetworkGroup's frame and legend tab, tinted with the role colour, and adds
 * a node/task summary; the tab's ☰ opens the group's panel. Fixed in place
 * like the network view's groups: dragging on it pans the canvas.
 */
export const RoleGroup = memo(function RoleGroup({ data }: NodeProps) {
  const { role, nodeCount, taskCount, onInfoClick } = data as unknown as RoleGroupData;
  const { theme } = useTheme();
  const color = swarmRoleColor(role, theme.mode);
  const title = roleTitle(role);
  const textColor = `${color}${theme.groupTextAlpha}`;

  return (
    <div
      data-testid="role-group"
      data-role={role}
      title={`${title}: ${formatCount(nodeCount, 'node')}, ${formatCount(taskCount, 'task')}`}
      style={{
        width: '100%',
        height: '100%',
        boxSizing: 'border-box',
        border: `1px solid ${color}${theme.groupBorderAlpha}`,
        borderRadius: 10,
        background: `${color}${theme.groupBgAlpha}`,
        position: 'relative',
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
          color: textColor,
          letterSpacing: '0.04em',
          textTransform: 'uppercase' as const,
          whiteSpace: 'nowrap',
        }}
      >
        {onInfoClick && (
          <InspectButton
            label={`Inspect ${title}`}
            title={`Inspect ${title.toLowerCase()}`}
            color={textColor}
            onClick={() => onInfoClick(roleGroupId(role))}
          />
        )}
        <span
          aria-hidden="true"
          style={{ width: 6, height: 6, borderRadius: '50%', background: color, flex: '0 0 auto' }}
        />
        <span>{title}</span>
        <span
          data-testid="role-group-summary"
          style={{ color: theme.nodeSubtext, fontWeight: 500, textTransform: 'none', letterSpacing: 0 }}
        >
          {formatCount(nodeCount, 'node')} · {formatCount(taskCount, 'task')}
        </span>
      </div>
    </div>
  );
});
