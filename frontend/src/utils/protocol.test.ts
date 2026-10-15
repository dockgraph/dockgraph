// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { PROTOCOL_VERSION, protocolAction } from './protocol';

describe('protocolAction', () => {
  beforeEach(() => sessionStorage.clear());

  it('accepts the matching version', () => {
    expect(protocolAction(PROTOCOL_VERSION, sessionStorage)).toBe('accept');
  });

  it('reloads once on a mismatched version, then ignores it', () => {
    expect(protocolAction(PROTOCOL_VERSION + 1, sessionStorage)).toBe('reload');
    expect(protocolAction(PROTOCOL_VERSION + 1, sessionStorage)).toBe('ignore');
  });

  it('rejects the pre-swarm version 1 payload', () => {
    expect(protocolAction(1, sessionStorage)).toBe('reload');
  });

  it('re-arms the reload after a matching version is seen', () => {
    protocolAction(PROTOCOL_VERSION + 1, sessionStorage);
    protocolAction(PROTOCOL_VERSION, sessionStorage);
    expect(protocolAction(PROTOCOL_VERSION + 1, sessionStorage)).toBe('reload');
  });

  it('ignores a mismatch without storage, so a reload cannot loop', () => {
    expect(protocolAction(PROTOCOL_VERSION + 1, null)).toBe('ignore');
    expect(protocolAction(PROTOCOL_VERSION, null)).toBe('accept');
  });

  it('ignores a mismatch when storage throws, and never throws itself', () => {
    const throwing = {
      getItem: () => { throw new Error('SecurityError'); },
      setItem: () => { throw new Error('QuotaExceededError'); },
      removeItem: () => { throw new Error('SecurityError'); },
    } as unknown as Storage;
    expect(protocolAction(PROTOCOL_VERSION + 1, throwing)).toBe('ignore');
    expect(protocolAction(PROTOCOL_VERSION, throwing)).toBe('accept');
  });

  it('ignores a mismatch when the marker write fails', () => {
    const readOnly = {
      getItem: () => null,
      setItem: () => { throw new Error('QuotaExceededError'); },
      removeItem: () => {},
    } as unknown as Storage;
    expect(protocolAction(PROTOCOL_VERSION + 1, readOnly)).toBe('ignore');
  });

  it('ignores a mismatch when the marker write does not stick', () => {
    const forgetful = { getItem: () => null, setItem: () => {}, removeItem: () => {} } as unknown as Storage;
    expect(protocolAction(PROTOCOL_VERSION + 1, forgetful)).toBe('ignore');
  });
});
