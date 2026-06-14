// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { ThemeProvider, useTheme } from './theme';
import type { ReactNode } from 'react';

const wrapper = ({ children }: { children: ReactNode }) => <ThemeProvider>{children}</ThemeProvider>;

describe('window theme tokens', () => {
  it('exposes window chrome tokens', () => {
    const { result } = renderHook(() => useTheme(), { wrapper });
    const t = result.current.theme;
    for (const key of ['windowBg', 'windowBorder', 'windowShadow', 'tabActiveBg', 'dropIndicator'] as const) {
      expect(typeof t[key]).toBe('string');
      expect(t[key].length).toBeGreaterThan(0);
    }
  });
});
