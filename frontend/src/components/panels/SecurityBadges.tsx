import { useTheme } from '../../theme';
import { Section } from './shared';

interface Props {
  privileged?: boolean;
  readonlyRootfs?: boolean;
  capAdd?: string[];
  capDrop?: string[];
}

/** Renders security-related badges: privileged, read-only, capabilities. */
export function SecurityBadges({ privileged, readonlyRootfs, capAdd, capDrop }: Props) {
  const { theme } = useTheme();
  const hasContent = privileged || readonlyRootfs || capAdd?.length || capDrop?.length;
  if (!hasContent) return null;

  return (
    <Section title="Security">
      {privileged && <div style={{ fontSize: 11, color: '#ef4444', fontWeight: 600, marginBottom: 4 }}>Privileged Mode</div>}
      {readonlyRootfs && <div style={{ fontSize: 11, color: '#22c55e', marginBottom: 4 }}>Read-only Root Filesystem</div>}
      {capAdd && capAdd.length > 0 && (
        <div style={{ fontSize: 10, color: '#f59e0b', marginBottom: 2 }}>
          +Capabilities: {capAdd.join(', ')}
        </div>
      )}
      {capDrop && capDrop.length > 0 && (
        <div style={{ fontSize: 10, color: theme.nodeSubtext }}>
          -Capabilities: {capDrop.join(', ')}
        </div>
      )}
    </Section>
  );
}
