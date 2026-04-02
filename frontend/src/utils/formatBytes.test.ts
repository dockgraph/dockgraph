import { describe, it, expect } from 'vitest';
import { formatBytes, formatBytesShort } from './formatBytes';

describe('formatBytes', () => {
  it('returns "0 B" for zero bytes', () => {
    expect(formatBytes(0)).toBe('0 B');
  });

  it('formats values below 1 KB as bytes', () => {
    expect(formatBytes(512)).toBe('512.0 B');
    expect(formatBytes(1)).toBe('1.0 B');
  });

  it('formats exact KB boundary (1024)', () => {
    expect(formatBytes(1024)).toBe('1.0 KB');
  });

  it('formats exact MB boundary (1048576)', () => {
    expect(formatBytes(1048576)).toBe('1.0 MB');
  });

  it('formats exact GB boundary (1073741824)', () => {
    expect(formatBytes(1073741824)).toBe('1.0 GB');
  });

  it('formats exact TB boundary (1099511627776)', () => {
    expect(formatBytes(1099511627776)).toBe('1.0 TB');
  });

  it('formats fractional values', () => {
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(2621440)).toBe('2.5 MB');
  });

  it('respects custom decimal precision', () => {
    expect(formatBytes(1536, 0)).toBe('2 KB');
    expect(formatBytes(1536, 2)).toBe('1.50 KB');
    expect(formatBytes(1536, 3)).toBe('1.500 KB');
  });

  it('handles large numbers near TB range', () => {
    const nearTB = 999 * 1024 * 1024 * 1024;
    expect(formatBytes(nearTB)).toBe('999.0 GB');

    const multiTB = 5.5 * 1024 * 1024 * 1024 * 1024;
    expect(formatBytes(multiTB)).toBe('5.5 TB');
  });

  it('clamps at TB for values beyond TB', () => {
    const hugeTB = 1024 * 1099511627776;
    expect(formatBytes(hugeTB)).toBe('1024.0 TB');
  });
});

describe('formatBytesShort', () => {
  it('returns "0B" for zero bytes', () => {
    expect(formatBytesShort(0)).toBe('0B');
  });

  it('formats exact KB boundary as "1.0K"', () => {
    expect(formatBytesShort(1024)).toBe('1.0K');
  });

  it('values less than 10 get 1 decimal place', () => {
    expect(formatBytesShort(1536)).toBe('1.5K');
    expect(formatBytesShort(1.5 * 1048576)).toBe('1.5M');
    expect(formatBytesShort(9.9 * 1048576)).toBe('9.9M');
  });

  it('values at 10 or above get rounded to integer', () => {
    expect(formatBytesShort(12 * 1048576)).toBe('12M');
    expect(formatBytesShort(100 * 1048576)).toBe('100M');
    expect(formatBytesShort(512 * 1048576)).toBe('512M');
  });

  it('handles the boundary at exactly 10', () => {
    expect(formatBytesShort(10 * 1024)).toBe('10K');
    expect(formatBytesShort(10 * 1048576)).toBe('10M');
  });

  it('formats byte values below 1K', () => {
    expect(formatBytesShort(500)).toBe('500B');
    expect(formatBytesShort(5)).toBe('5.0B');
  });

  it('formats GB and TB values', () => {
    expect(formatBytesShort(2 * 1073741824)).toBe('2.0G');
    expect(formatBytesShort(50 * 1073741824)).toBe('50G');
    expect(formatBytesShort(1099511627776)).toBe('1.0T');
  });
});
