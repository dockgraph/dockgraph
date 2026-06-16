// @vitest-environment jsdom
import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { ThemeProvider } from '../theme';
import { SearchFilter } from './SearchFilter';
import type { SearchFilterResult } from '../hooks/useSearchFilter';

afterEach(() => cleanup());

const mockSearch = (): SearchFilterResult => ({
  query: '',
  setQuery: vi.fn(),
  filters: { statuses: new Set<string>(), types: new Set<string>() },
  toggleStatus: vi.fn(),
  toggleType: vi.fn(),
  clearAll: vi.fn(),
  matchingNodeIds: null,
  hasActiveFilter: false,
  totalCount: 0,
  matchCount: 0,
});

describe('SearchFilter', () => {
  it('lets the field shrink below its preferred width so it cannot overlap the header tabs/status', () => {
    render(
      <ThemeProvider>
        <SearchFilter search={mockSearch()} />
      </ThemeProvider>,
    );

    const input = screen.getByLabelText('Search containers, images, and labels');
    const box = input.parentElement as HTMLElement;

    // 320px is only a preferred width — it must be able to shrink within a narrow header.
    expect(box.style.width).toBe('320px');
    expect(box.style.maxWidth).toBe('100%');
    expect(box.style.minWidth).toBe('0px');

    const root = box.parentElement as HTMLElement;
    expect(root.style.minWidth).toBe('0px');
  });
});
