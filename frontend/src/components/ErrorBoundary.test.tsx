// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ErrorBoundary } from './ErrorBoundary';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

function ProblemChild({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) throw new Error('render failure');
  return <div>child content</div>;
}

describe('ErrorBoundary', () => {
  it('renders children when no error occurs', () => {
    render(
      <ErrorBoundary>
        <div>healthy child</div>
      </ErrorBoundary>,
    );

    expect(screen.getByText('healthy child')).toBeDefined();
  });

  it('shows error UI when a child throws during render', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});

    render(
      <ErrorBoundary>
        <ProblemChild shouldThrow />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Something went wrong rendering the graph.')).toBeDefined();
    expect(screen.getByText('Retry')).toBeDefined();
  });

  it('clears error state when retry is clicked', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});

    let throwOnRender = true;
    function ConditionalThrower() {
      if (throwOnRender) throw new Error('bang');
      return <div>recovered</div>;
    }

    render(
      <ErrorBoundary>
        <ConditionalThrower />
      </ErrorBoundary>,
    );

    expect(screen.getByText('Something went wrong rendering the graph.')).toBeDefined();

    // Stop throwing, then click retry
    throwOnRender = false;
    fireEvent.click(screen.getByText('Retry'));

    expect(screen.getByText('recovered')).toBeDefined();
  });

  it('catches the error details in componentDidCatch', () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    render(
      <ErrorBoundary>
        <ProblemChild shouldThrow />
      </ErrorBoundary>,
    );

    // React 19 logs to console.error for caught errors
    expect(errorSpy).toHaveBeenCalled();
  });
});
