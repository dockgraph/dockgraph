import { useTheme } from '../../theme';
import { Section } from './shared';
import type { ContainerDetail } from '../../types/stats';

interface Props {
  env: ContainerDetail['env'];
}

export function DetailPanelEnv({ env }: Props) {
  const { theme } = useTheme();
  if (!env?.length) return null;

  return (
    <Section title="Environment">
      {env.map((e, i) => {
        const masked = e.value === '********';
        return (
          <div key={i} style={{ fontSize: 11, marginBottom: 3, display: 'flex', gap: 4, alignItems: 'baseline' }}>
            <span style={{ color: theme.nodeSubtext, flexShrink: 0 }}>{e.key}=</span>
            <span style={{ fontFamily: 'monospace', color: masked ? theme.nodeSubtext : theme.panelText, wordBreak: 'break-all', minWidth: 0 }}>
              {masked ? '••••••••' : e.value}
            </span>
          </div>
        );
      })}
    </Section>
  );
}
