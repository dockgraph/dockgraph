// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useNetworkDetail } from './useNetworkDetail';
import { useResourceDetail } from './useResourceDetail';

vi.mock('./useResourceDetail', () => ({ useResourceDetail: vi.fn(() => ({ data: null, loading: false, error: null })) }));

describe('useNetworkDetail', () => {
  beforeEach(() => vi.mocked(useResourceDetail).mockClear());

  it('has no url without a network', () => {
    renderHook(() => useNetworkDetail(null));
    expect(useResourceDetail).toHaveBeenCalledWith(null);
  });

  it('asks the server for the network by name', () => {
    renderHook(() => useNetworkDetail('app_net'));
    expect(useResourceDetail).toHaveBeenCalledWith('/api/networks/app_net');
  });

  it('names the owner of a node-local network', () => {
    renderHook(() => useNetworkDetail('lan', 'mgr b'));
    expect(useResourceDetail).toHaveBeenCalledWith('/api/networks/lan?node=mgr%20b');
  });
});
