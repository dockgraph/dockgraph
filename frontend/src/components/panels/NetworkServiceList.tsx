import { useState } from 'react';
import { useTheme } from '../../theme';
import { Section, Row, navLinkStyle, navButtonReset, monoStyle } from './shared';
import { panelTableStyles } from './panelStyles';
import { Copyable } from './Copyable';
import { taskSlot } from '../../utils/nodeTransform';
import type { NetworkService } from '../../types/stats';

interface Props {
  services: NetworkService[];
  /** The swarm couldn't be asked: say so instead of listing services. */
  unavailable: boolean;
  onNavigate?: (nodeId: string) => void;
}

/**
 * The services attached to a swarm network across the cluster. Each links to
 * its graph node and shows its VIP; its tasks, one per replica or node, fold
 * out on demand so global services don't flood the panel.
 */
export function NetworkServiceList({ services, unavailable, onNavigate }: Props) {
  const { theme } = useTheme();

  return (
    <Section title={unavailable ? 'Connected Services' : `Connected Services (${services.length})`}>
      {unavailable ? (
        <div style={{ fontSize: 11, color: theme.nodeSubtext }}>
          The network's services couldn't be listed from the swarm. The containers below are only those on this node.
        </div>
      ) : services.length === 0 ? (
        <div style={{ fontSize: 11, color: theme.nodeSubtext }}>No services.</div>
      ) : (
        services.map((s) => <NetworkServiceEntry key={s.name} service={s} onNavigate={onNavigate} />)
      )}
    </Section>
  );
}

function NetworkServiceEntry({ service, onNavigate }: { service: NetworkService; onNavigate?: (nodeId: string) => void }) {
  const { theme } = useTheme();
  const [open, setOpen] = useState(false);
  const mono = monoStyle(theme.panelText);
  const count = service.tasks.length;
  const tasksText = `${count} ${count === 1 ? 'task' : 'tasks'}`;
  // Global services' tasks have no slot: they are told apart by their node.
  const slotted = service.tasks.some((t) => taskSlot(t, service.mode) !== undefined);

  const table = panelTableStyles(theme.nodeSubtext);
  const cell: React.CSSProperties = { ...table.cell, fontFamily: 'var(--dg-font-mono)' };
  const head = table.head;
  const name: React.CSSProperties = { fontWeight: 600, color: theme.panelText, textAlign: 'left', wordBreak: 'break-all' };

  return (
    <div style={{ marginBottom: 8 }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, fontSize: 11, marginBottom: 3 }}>
        {onNavigate ? (
          <button
            type="button"
            style={{ ...navButtonReset, ...name, ...navLinkStyle(theme.panelBorder) }}
            title={`Inspect ${service.name}`}
            onClick={() => onNavigate(`service:${service.name}`)}
          >
            {service.name}
          </button>
        ) : (
          <span style={name}>{service.name}</span>
        )}
        <button
          type="button"
          aria-expanded={open}
          aria-label={`${tasksText} of ${service.name}`}
          onClick={() => setOpen((o) => !o)}
          style={{
            ...navButtonReset,
            marginLeft: 'auto',
            flexShrink: 0,
            display: 'inline-flex',
            alignItems: 'center',
            gap: 5,
            cursor: 'pointer',
            fontFamily: 'var(--dg-font-mono)',
            fontSize: 10.5,
            color: theme.nodeSubtext,
          }}
        >
          <span
            aria-hidden="true"
            style={{ display: 'inline-block', fontSize: 8, transition: 'transform 0.15s', transform: open ? 'rotate(90deg)' : 'rotate(0deg)' }}
          >
            ▶
          </span>
          {tasksText}
        </button>
      </div>
      {service.vip && <Row label="VIP" value={service.vip} mono={mono} subtext={theme.nodeSubtext} />}
      {open && count > 0 && (
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 11, color: theme.panelText, marginTop: 2 }}>
          <thead>
            <tr>
              {slotted && <th style={head}>Slot</th>}
              <th style={head}>Node</th>
              <th style={head}>Address</th>
            </tr>
          </thead>
          <tbody>
            {service.tasks.map((t) => (
              <tr key={t.id} style={{ borderTop: `1px solid ${theme.panelBorder}` }}>
                {slotted && <td style={cell}>{taskSlot(t, service.mode) ?? '—'}</td>}
                <td style={{ ...cell, wordBreak: 'break-all' }} title={t.nodeId}>
                  {t.nodeHostname || t.nodeId?.slice(0, 12) || '—'}
                </td>
                <td style={cell}>
                  {t.addresses && t.addresses.length > 0 ? <Copyable value={t.addresses.join(', ')} /> : '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
