import { memo, useCallback } from 'react';

interface Props {
  label: string;
  title: string;
  color: string;
  onClick: () => void;
}

/** Three-bar hamburger button used on container, volume, and network nodes. */
export const InspectButton = memo(function InspectButton({ label, title, color, onClick }: Props) {
  const handleClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    onClick();
  }, [onClick]);

  return (
    <button
      onClick={handleClick}
      aria-label={label}
      title={title}
      style={{
        width: 14,
        height: 14,
        borderRadius: 2,
        border: 'none',
        background: 'transparent',
        cursor: 'pointer',
        padding: 0,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'flex-end',
        justifyContent: 'center',
        gap: 2,
        flexShrink: 0,
      }}
    >
      <span style={{ width: 10, height: 2, background: color, borderRadius: 1, display: 'block' }} />
      <span style={{ width: 10, height: 2, background: color, borderRadius: 1, display: 'block' }} />
      <span style={{ width: 7, height: 2, background: color, borderRadius: 1, display: 'block' }} />
    </button>
  );
});
