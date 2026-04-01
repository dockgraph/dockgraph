import { useTheme } from '../../theme';
import { Section } from './shared';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  security: ContainerDetail['security'];
}

export function DetailPanelSecurity({ security }: Props) {
  const { theme } = useTheme();
  if (!security) return null;

  const hasContent = security.privileged || security.readonlyRootfs || security.capAdd?.length || security.capDrop?.length;
  if (!hasContent) return null;

  return (
    <Section title="Security">
      {security.privileged && <div style={{ fontSize: 11, color: '#ef4444', fontWeight: 600, marginBottom: 4 }}>Privileged Mode</div>}
      {security.readonlyRootfs && <div style={{ fontSize: 11, color: '#22c55e', marginBottom: 4 }}>Read-only Root Filesystem</div>}
      {security.capAdd?.length > 0 && (
        <div style={{ fontSize: 10, color: '#f59e0b', marginBottom: 2 }}>
          +Capabilities: {security.capAdd.join(', ')}
        </div>
      )}
      {security.capDrop?.length > 0 && (
        <div style={{ fontSize: 10, color: theme.nodeSubtext }}>
          -Capabilities: {security.capDrop.join(', ')}
        </div>
      )}
    </Section>
  );
}
