import { useTheme } from '../../theme';
import { Section, Row, navLinkStyle, monoStyle } from './shared';
import { SecurityBadges } from './SecurityBadges';
import { DetailPanelMounts } from './DetailPanelMounts';
import { DetailPanelEnv } from './DetailPanelEnv';
import { DetailPanelLabels } from './DetailPanelLabels';
import type { ComposeConfig } from '../../types';

interface Props {
  compose: ComposeConfig;
  onNavigate?: (targetId: string) => void;
}

export function DetailPanelCompose({ compose, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);

  return (
    <>
      <Section title="Process">
        {compose.command && compose.command.length > 0 && <Row label="Command" value={compose.command.join(' ')} mono={mono} subtext={theme.nodeSubtext} />}
        {compose.entrypoint && compose.entrypoint.length > 0 && <Row label="Entrypoint" value={compose.entrypoint.join(' ')} mono={mono} subtext={theme.nodeSubtext} />}
        {compose.workingDir && <Row label="Working Dir" value={compose.workingDir} mono={mono} subtext={theme.nodeSubtext} />}
        {compose.user && <Row label="User" value={compose.user} mono={mono} subtext={theme.nodeSubtext} />}
        {compose.restart && <Row label="Restart" value={compose.restart} mono={mono} subtext={theme.nodeSubtext} />}
      </Section>

      {compose.dependsOn && compose.dependsOn.length > 0 && (
        <Section title="Dependencies">
          {compose.dependsOn.map((dep) => (
            <div
              key={dep}
              style={{
                fontSize: 11,
                color: theme.panelText,
                marginBottom: 2,
                fontFamily: 'var(--dg-font-mono)',
                ...(onNavigate ? navLinkStyle(theme.panelBorder) : {}),
              }}
              onClick={onNavigate ? () => onNavigate(`container:${dep}`) : undefined}
              title={onNavigate ? `Inspect ${dep}` : undefined}
            >
              {dep}
            </div>
          ))}
        </Section>
      )}

      <DetailPanelMounts mounts={compose.volumes ?? []} onNavigate={onNavigate} />


      {compose.networks && compose.networks.length > 0 && (
        <Section title="Networks">
          {compose.networks.map((n) => (
            <div
              key={n}
              style={{
                fontSize: 11,
                color: theme.panelText,
                marginBottom: 2,
                fontFamily: 'var(--dg-font-mono)',
                ...(onNavigate ? navLinkStyle(theme.panelBorder) : {}),
              }}
              onClick={onNavigate ? () => onNavigate(`network:${n}`) : undefined}
              title={onNavigate ? `Inspect ${n}` : undefined}
            >
              {n}
            </div>
          ))}
        </Section>
      )}

      <DetailPanelEnv env={Object.entries(compose.environment ?? {}).map(([key, value]) => ({ key, value }))} />

      <DetailPanelLabels labels={compose.labels} />

      <SecurityBadges
        privileged={compose.privileged}
        readonlyRootfs={compose.readOnly}
        capAdd={compose.capAdd}
        capDrop={compose.capDrop}
      />
    </>
  );
}

