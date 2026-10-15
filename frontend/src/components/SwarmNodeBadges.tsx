import { memo } from 'react';
import { useTheme } from '../theme';
import { swarmRoleColor, swarmNodeStateColor } from '../utils/colors';
import { chip } from '../utils/chipStyle';
import type { SwarmNodeInfo } from '../types';

/** "manager ★" (leader) / "manager" / "worker" chip. */
export const SwarmRoleBadge = memo(function SwarmRoleBadge({ info }: { info: SwarmNodeInfo | undefined }) {
  const { theme } = useTheme();
  if (!info) return null;
  const manager = info.role === 'manager';
  const color = swarmRoleColor(manager ? 'manager' : 'worker', theme.mode);
  return (
    <span
      data-testid="role-badge"
      title={info.leader ? 'Manager (leader)' : manager ? 'Manager' : 'Worker'}
      style={{
        ...chip,
        color,
        border: `1px solid ${color}55`,
        background: manager ? `${color}1f` : 'transparent',
      }}
    >
      {info.role}{info.leader ? ' ★' : ''}
    </span>
  );
});

/** Availability chip, shown only when the node is paused or drained. */
export const SwarmAvailabilityChip = memo(function SwarmAvailabilityChip({ info }: { info: SwarmNodeInfo | undefined }) {
  const { theme } = useTheme();
  if (!info || !info.availability || info.availability === 'active') return null;
  const color = info.availability === 'drain' ? theme.danger : theme.warning;
  return (
    <span
      data-testid="availability-chip"
      title={`Availability: ${info.availability}`}
      style={{ ...chip, color, border: `1px solid ${color}88`, background: `${color}1f` }}
    >
      {info.availability}
    </span>
  );
});

/** State chip, shown only when the node is not ready (down, disconnected...). */
export const SwarmStateChip = memo(function SwarmStateChip({ state }: { state: string | undefined }) {
  if (!state || state === 'ready') return null;
  const color = swarmNodeStateColor(state);
  return (
    <span
      data-testid="state-chip"
      title={`State: ${state}`}
      style={{ ...chip, color, border: `1px solid ${color}88`, background: `${color}1f` }}
    >
      {state}
    </span>
  );
});
