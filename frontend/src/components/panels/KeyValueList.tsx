import { useTheme } from '../../theme';
import { monoStyle } from './panelStyles';
import { Copyable } from './Copyable';

interface Props {
  entries: Record<string, string>;
}

/** Renders a Record<string, string> as key=value rows with click-to-copy values. */
export function KeyValueList({ entries }: Props) {
  const { theme } = useTheme();
  const mono = monoStyle(theme.panelText);

  return (
    <>
      {Object.entries(entries).map(([k, v]) => (
        <div key={k} style={{ fontSize: 11, marginBottom: 2 }}>
          <span style={{ color: theme.nodeSubtext }}>{k}=</span>
          <Copyable value={v} style={{ ...mono, wordBreak: 'break-all' }}>{v}</Copyable>
        </div>
      ))}
    </>
  );
}
