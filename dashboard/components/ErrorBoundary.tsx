import React from 'react';

type ErrorBoundaryProps = {
  children: React.ReactNode;
};

type ErrorBoundaryState = {
  error: Error | null;
};

export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    // Keep this visible in production logs without exposing stack traces in the UI.
    console.error('[Fortuna] render error boundary caught', error, info.componentStack);
  }

  private retry = () => {
    this.setState({ error: null });
  };

  private goDashboard = () => {
    this.setState({ error: null }, () => {
      window.location.hash = '#/dashboard';
    });
  };

  render() {
    if (!this.state.error) return this.props.children;

    return (
      <main className="flex min-h-dvh items-center justify-center bg-base px-4 py-10 text-text" role="alert">
        <section className="w-full max-w-xl rounded-xl border border-border bg-surface p-6 shadow-2xl">
          <p className="text-meta font-semibold uppercase tracking-wide text-critical">Interface recovery</p>
          <h1 className="mt-2 text-page-title">Something went wrong loading this view</h1>
          <p className="mt-3 text-body text-muted">
            Fortuna caught a dashboard rendering error before it could blank the workspace. Try the view again, return to
            the dashboard, or reload if the problem persists.
          </p>
          <details className="mt-4 rounded-lg border border-border bg-base/60 p-3 text-caption text-muted">
            <summary className="cursor-pointer text-text">Technical detail</summary>
            <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap break-words">
              {this.state.error.message}
            </pre>
          </details>
          <div className="mt-5 flex flex-col gap-2 sm:flex-row">
            <button
              type="button"
              onClick={this.retry}
              className="rounded-md bg-brand px-4 py-2 text-body font-semibold text-white transition-colors hover:bg-brand-strong focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
            >
              Try again
            </button>
            <button
              type="button"
              onClick={this.goDashboard}
              className="rounded-md border border-border bg-surface-2 px-4 py-2 text-body font-semibold text-text transition-colors hover:bg-surface focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
            >
              Go to dashboard
            </button>
            <button
              type="button"
              onClick={() => window.location.reload()}
              className="rounded-md border border-border bg-transparent px-4 py-2 text-body font-semibold text-muted transition-colors hover:bg-surface-2 hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
            >
              Reload app
            </button>
          </div>
        </section>
      </main>
    );
  }
}
