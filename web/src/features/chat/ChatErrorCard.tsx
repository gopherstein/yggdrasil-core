import { useState } from 'react'
import { Link } from 'react-router-dom'
import { explainError } from './friendlyError'

/** A chat error in plain language, with the next step and the details on request. */
export function ChatErrorCard({ raw, onRetry, onNewChat }: { raw: string; onRetry?: () => void; onNewChat?: () => void }) {
  const [showDetail, setShowDetail] = useState(false)
  const e = explainError(raw)
  return (
    <div role="alert" className="max-w-[min(42rem,85%)] rounded-2xl border border-danger/25 bg-danger/5 px-4 py-3 text-sm">
      <p className="font-medium text-ink">{e.title}</p>
      <p className="mt-0.5 text-ink-muted">{e.body}</p>
      <div className="mt-2.5 flex flex-wrap items-center gap-2">
        {e.actions.includes('retry') && onRetry ? (
          <button type="button" className="btn-primary px-3 py-1 text-xs" onClick={onRetry}>
            Try again
          </button>
        ) : null}
        {e.actions.includes('new-chat') && onNewChat ? (
          <button type="button" className="btn-secondary px-3 py-1 text-xs" onClick={onNewChat}>
            Start a new chat
          </button>
        ) : null}
        {e.actions.includes('models') ? (
          <Link to="/models" className="btn-secondary px-3 py-1 text-xs">
            Open Models
          </Link>
        ) : null}
        {e.actions.includes('computers') ? (
          <Link to="/nodes" className="btn-secondary px-3 py-1 text-xs">
            Open Computers
          </Link>
        ) : null}
        {e.detail ? (
          <button type="button" className="ml-auto text-xs text-ink-faint hover:text-ink" onClick={() => setShowDetail((v) => !v)}>
            {showDetail ? 'Hide details' : 'Details'}
          </button>
        ) : null}
      </div>
      {showDetail && e.detail ? <pre className="log-panel mt-2 whitespace-pre-wrap break-words text-xs">{e.detail}</pre> : null}
    </div>
  )
}
