import { Component, type ErrorInfo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

function PageError({ error, onRetry }: { error: Error; onRetry: () => void }) {
  const { t } = useTranslation()
  return (
    <div role="alert" className="empty-hero max-w-lg">
      <h1 className="font-display text-xl font-semibold text-ink">{t('pageError.title')}</h1>
      <p className="text-[15px] leading-relaxed text-ink-muted">{t('pageError.body')}</p>
      <div className="flex flex-wrap gap-2 pt-2">
        <button type="button" className="btn-primary" onClick={onRetry}>
          {t('pageError.retry')}
        </button>
        <button type="button" className="btn-secondary" onClick={() => window.location.reload()}>
          {t('pageError.reload')}
        </button>
      </div>
      <details className="pt-2 text-xs text-ink-faint">
        <summary className="cursor-pointer">{t('pageError.details')}</summary>
        <pre className="mt-2 whitespace-pre-wrap break-all font-mono">{error.message}</pre>
      </details>
    </div>
  )
}

interface Props {
  children: ReactNode
}

interface State {
  error: Error | null
}

/**
 * Keeps one page's error to that page: it shows what happened and a way to
 * try again, and the sidebar keeps working, where the whole app used to go
 * blank. AppLayout keys it by path, so moving to another page starts fresh.
 */
export class PageErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('page error', error, info.componentStack)
  }

  render() {
    if (this.state.error) {
      return <PageError error={this.state.error} onRetry={() => this.setState({ error: null })} />
    }
    return this.props.children
  }
}
