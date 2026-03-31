// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { LogoutButton } from './LogoutButton';
import { ThemeProvider } from '../theme';

describe('LogoutButton', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders nothing when auth is disabled (404)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(null, { status: 404 }),
    );
    const { container } = render(
      <ThemeProvider><LogoutButton /></ThemeProvider>,
    );
    await waitFor(() => {
      expect(container.innerHTML).toBe('');
    });
  });

  it('renders sign out button when auth is enabled (200 JSON)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('{"authenticated":true}', {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    render(<ThemeProvider><LogoutButton /></ThemeProvider>);
    await waitFor(() => {
      expect(screen.getByTitle('Sign out')).toBeDefined();
    });
  });

  it('renders nothing when 200 but not JSON (SPA fallback)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('<html>app</html>', {
        status: 200,
        headers: { 'Content-Type': 'text/html' },
      }),
    );
    const { container } = render(
      <ThemeProvider><LogoutButton /></ThemeProvider>,
    );
    await waitFor(() => {
      expect(container.innerHTML).toBe('');
    });
  });
});
