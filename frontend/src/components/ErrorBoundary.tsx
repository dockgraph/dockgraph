import { Component, type ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

/**
 * Catches rendering errors in the component tree and displays a recovery UI
 * instead of white-screening the entire application.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info.componentStack);
  }

  private handleRetry = () => {
    this.setState({ error: null });
  };

  render() {
    if (this.state.error) {
      return (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            height: '100%',
            gap: 12,
            color: '#94a3b8',
            fontFamily: 'system-ui, sans-serif',
          }}
        >
          <p style={{ fontSize: 14 }}>Something went wrong rendering the graph.</p>
          <button
            onClick={this.handleRetry}
            style={{
              padding: '6px 16px',
              fontSize: 13,
              borderRadius: 4,
              border: '1px solid #475569',
              background: 'transparent',
              color: '#94a3b8',
              cursor: 'pointer',
            }}
          >
            Retry
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}
