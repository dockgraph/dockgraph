import { useTheme } from '../../theme';
import { Section } from './shared';

interface Props {
  labels: Record<string, string> | undefined;
}

export function DetailPanelLabels({ labels }: Props) {
  const { theme } = useTheme();
  const entries = labels ? Object.entries(labels) : [];
  if (!entries.length) return null;

  return (
    <Section title="Labels">
      {entries.map(([key, value]) => (
        <div key={key} style={{ fontSize: 11, marginBottom: 3 }}>
          <span style={{ color: theme.nodeSubtext }}>{key}: </span>
          <span style={{ fontFamily: 'monospace', color: theme.panelText, wordBreak: 'break-all' }}>{value}</span>
        </div>
      ))}
    </Section>
  );
}
