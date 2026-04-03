import { useTheme } from '../../theme';
import { Section, Row, navLinkStyle, monoStyle } from './shared';
import { SecurityBadges } from './SecurityBadges';
import { KeyValueList } from './KeyValueList';
import type { ComposeConfig } from '../../types';

interface Props {
  compose: ComposeConfig;
  image?: string;
  onNavigate?: (targetId: string) => void;
}

export function DetailPanelCompose({ compose, image, onNavigate }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);

  return (
    <>
      <Section title="Service Configuration">
        {image && <Row label="Image" value={image} mono={mono} subtext={theme.nodeSubtext} />}
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
                fontFamily: 'monospace',
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

      {compose.volumes && compose.volumes.length > 0 && (
        <Section title="Volumes">
          {compose.volumes.map((v, i) => (
            <div key={i} style={{ fontSize: 11, color: theme.panelText, marginBottom: 2, fontFamily: 'monospace', wordBreak: 'break-all' }}>{v}</div>
          ))}
        </Section>
      )}

      {compose.networks && compose.networks.length > 0 && (
        <Section title="Networks">
          {compose.networks.map((n) => (
            <div
              key={n}
              style={{
                fontSize: 11,
                color: theme.panelText,
                marginBottom: 2,
                fontFamily: 'monospace',
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

      {compose.environment && Object.keys(compose.environment).length > 0 && (
        <Section title="Environment">
          <KeyValueList entries={compose.environment} />
        </Section>
      )}

      <SecurityBadges
        privileged={compose.privileged}
        readonlyRootfs={compose.readOnly}
        capAdd={compose.capAdd}
        capDrop={compose.capDrop}
      />
    </>
  );
}

