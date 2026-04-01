import { useTheme } from '../../theme';
import { Section, navLinkStyle } from './shared';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  mounts: ContainerDetail['mounts'];
  onNavigate?: (nodeId: string) => void;
}

export function DetailPanelMounts({ mounts, onNavigate }: Props) {
  const { theme } = useTheme();
  if (!mounts?.length) return null;

  return (
    <Section title="Mounts">
      {mounts.map((m, i) => {
        const volumeName = m.type === 'volume' ? m.name : undefined;
        const clickable = !!volumeName && !!onNavigate;
        return (
          <div key={i} style={{ fontSize: 11, color: theme.panelText, marginBottom: 4 }}>
            <div
              style={{
                fontFamily: 'monospace',
                wordBreak: 'break-all',
                ...(clickable ? navLinkStyle(theme.panelBorder) : {}),
              }}
              title={clickable ? `Inspect volume ${volumeName}` : m.source}
              onClick={clickable ? () => onNavigate(`volume:${volumeName}`) : undefined}
            >
              {volumeName ?? m.source}
            </div>
            <div style={{ fontSize: 10, color: theme.nodeSubtext }}>
              → {m.destination} ({m.type}{m.rw ? '' : ', ro'})
            </div>
          </div>
        );
      })}
    </Section>
  );
}
