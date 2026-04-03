import { useTheme } from '../../theme';
import { navLinkStyle } from './panelStyles';

interface Props {
  name: string;
  onNavigate?: (targetId: string) => void;
}

/** Clickable container name that navigates to the container's detail panel. */
export function ContainerLink({ name, onNavigate }: Props) {
  const { theme } = useTheme();

  return (
    <div
      role={onNavigate ? 'button' : undefined}
      tabIndex={onNavigate ? 0 : undefined}
      style={{
        fontSize: 11,
        fontWeight: 600,
        color: theme.panelText,
        marginBottom: 4,
        ...(onNavigate ? navLinkStyle(theme.panelBorder) : {}),
      }}
      onClick={onNavigate ? () => onNavigate(`container:${name}`) : undefined}
      onKeyDown={onNavigate ? (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onNavigate(`container:${name}`); } } : undefined}
      title={onNavigate ? `Inspect ${name}` : undefined}
    >
      {name}
    </div>
  );
}
