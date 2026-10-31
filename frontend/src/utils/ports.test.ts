import { describe, it, expect } from 'vitest';
import { containerPorts, hostPorts, portMapping, portSpan } from './ports';

describe('portSpan', () => {
  it('writes one port', () => {
    expect(portSpan(8080)).toBe('8080');
  });

  it('writes a range as Docker does', () => {
    expect(portSpan(49152, 65535)).toBe('49152-65535');
  });
});

describe('hostPorts and containerPorts', () => {
  it('write each side of a range', () => {
    const p = { host: 8000, hostEnd: 8010, container: 80, containerEnd: 90 };
    expect(hostPorts(p)).toBe('8000-8010');
    expect(containerPorts(p)).toBe('80-90');
  });

  it('write a host range for one container port', () => {
    const p = { host: 8000, hostEnd: 8010, container: 80 };
    expect(hostPorts(p)).toBe('8000-8010');
    expect(containerPorts(p)).toBe('80');
  });
});

describe('portMapping', () => {
  it('writes host and container sides as Docker does', () => {
    expect(portMapping({ host: 8080, container: 80 })).toBe('8080:80');
    expect(portMapping({ host: 8000, hostEnd: 8010, container: 80, containerEnd: 90 })).toBe('8000-8010:80-90');
  });
});
