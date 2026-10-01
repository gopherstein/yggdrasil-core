import { useState } from 'react'
import { Ratatoskr } from '@/components/ui/Ratatoskr'
import type { ModelFailure } from './modelFailure'

export function ModelFailureNotice({
  failure,
  advanced,
  onRetry,
  onChooseModel,
  mascot = true,
}: {
  failure: ModelFailure
  advanced: boolean
  onRetry: () => void
  onChooseModel: () => void
  /** Show Ratatoskr with the acorn dropped. Off when another mascot is already in view. */
  mascot?: boolean
}) {
  const [detailsOpen, setDetailsOpen] = useState(false)
  return (
    <div className="flex max-w-[min(42rem,85%)] gap-3 rounded-2xl border border-line bg-raised/50 px-4 py-3 text-sm text-ink">
      {mascot ? <Ratatoskr state="error" size={48} className="-ml-1 mt-0.5" /> : null}
      <div className="min-w-0 flex-1">
        <p>{failure.message}</p>
        <div className="mt-3 flex flex-wrap gap-2">
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={onRetry}>
            Retry
          </button>
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={onChooseModel}>
            Use another assistant/model
          </button>
        </div>
        {advanced ? (
          <div className="mt-3">
            <button
              type="button"
              className="text-xs text-ink-muted underline-offset-2 hover:underline"
              onClick={() => setDetailsOpen((open) => !open)}
            >
              {detailsOpen ? 'Hide details' : 'Details'}
            </button>
            {detailsOpen ? (
              <dl className="mt-2 space-y-1 text-xs text-ink-muted">
                <div>Reason: {failure.reason}</div>
                {failure.runtime ? <div>Runtime: {failure.runtime}</div> : null}
                {failure.node_id ? <div>Computer: {failure.node_id}</div> : null}
                {failure.exit_code != null ? <div>Exit code: {failure.exit_code}</div> : null}
                {failure.last_probe ? <div>Last health probe: {failure.last_probe}</div> : null}
                {failure.stderr_tail ? (
                  <pre className="mt-1 max-h-32 overflow-auto whitespace-pre-wrap rounded-md bg-surface p-2">
                    {failure.stderr_tail}
                  </pre>
                ) : null}
              </dl>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  )
}
