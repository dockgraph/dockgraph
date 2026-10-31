import type { PortMapping } from '../types';

/** A port, or the range of ports from start to end, as Docker writes it: "8000-8010". */
export function portSpan(start: number, end?: number): string {
  return end ? `${start}-${end}` : String(start);
}

/** The host side of a mapping: one port or a range. */
export function hostPorts(p: PortMapping): string {
  return portSpan(p.host, p.hostEnd);
}

/** The container side of a mapping: one port or a range. */
export function containerPorts(p: PortMapping): string {
  return portSpan(p.container, p.containerEnd);
}

/** A mapping as Docker writes it: "8000-8010:80". */
export function portMapping(p: PortMapping): string {
  return `${hostPorts(p)}:${containerPorts(p)}`;
}
