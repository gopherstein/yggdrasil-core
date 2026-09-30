import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { formatBytes } from '@/lib/format'
import type { SpecializedAIView, TrainingJob } from '@/types/api'
import { elapsedSec, formatDuration, isTerminal, jobPercent, jobStages, stateLabel } from '../display'

export function TrainStep({ view, onNext }: { view: SpecializedAIView; onNext: () => void }) {
  const [now, setNow] = useState(() => Date.now())
  const job = view.jobs[0]
  const running = job && !isTerminal(job.state)
  useEffect(() => {
    if (!running) return
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [running])

  if (!job) {
    return <div className="card text-sm text-ink-muted">No training yet. Review the plan and start training.</div>
  }
  return (
    <div className="space-y-4">
      <JobCard job={job} now={now} />
      {job.state === 'complete' && (
        <button type="button" className="btn-primary px-3 py-1.5 text-sm" onClick={onNext}>
          Compare with the base model
        </button>
      )}
      {view.jobs.length > 1 && (
        <div className="card space-y-2">
          <h3 className="section-title">Earlier runs</h3>
          <ul className="divide-y divide-line/60 text-sm">
            {view.jobs.slice(1).map((j) => (
              <li key={j.id} className="flex flex-wrap items-center justify-between gap-2 py-1.5">
                <span className="text-ink">Revision {j.revision}</span>
                <span className="text-ink-muted">
                  {stateLabel(j.state)} · {formatDuration(elapsedSec(j))}
                  {j.error && <span className="text-danger"> · {j.error}</span>}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

function JobCard({ job, now }: { job: TrainingJob; now: number }) {
  const queryClient = useQueryClient()
  const cancel = useMutation({
    mutationFn: () => api.cancelTraining(job.id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['training'] }),
  })
  const p = job.progress
  const terminal = isTerminal(job.state)
  const currentIndex = jobStages.indexOf(job.state)
  const downloading = job.state === 'loading_model' && (p.download_total ?? 0) > 0 && (p.download_bytes ?? 0) < (p.download_total ?? 0)

  return (
    <div className="card space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h3 className="section-title">Revision {job.revision}</h3>
          <p className="text-xs text-ink-muted">
            On {job.node_name || job.node_id} with {job.backend.toUpperCase()} · {job.hyper.method?.toUpperCase()} ·{' '}
            {job.hyper.epochs} epochs
          </p>
        </div>
        {!terminal && (
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" disabled={cancel.isPending} onClick={() => cancel.mutate()}>
            {cancel.isPending ? 'Cancelling…' : 'Cancel'}
          </button>
        )}
      </div>

      <ol className="flex flex-wrap gap-1 text-xs" aria-label="Training stages">
        {jobStages.map((s, i) => {
          const state =
            job.state === 'failed' || job.state === 'cancelled'
              ? i < Math.max(currentIndex, 0)
                ? 'done'
                : 'todo'
              : i < currentIndex || job.state === 'complete'
                ? 'done'
                : i === currentIndex
                  ? 'now'
                  : 'todo'
          return (
            <li
              key={s}
              className={[
                'rounded-md px-2 py-1',
                state === 'done' ? 'bg-primary-soft text-primary-active' : state === 'now' ? 'bg-primary text-primary-fg' : 'bg-raised text-ink-faint',
              ].join(' ')}
            >
              {stateLabel(s)}
            </li>
          )
        })}
      </ol>

      {job.state === 'failed' && (
        <p className="rounded-md bg-danger/10 p-3 text-sm text-danger">
          {job.error || 'Training failed.'} Temporary files were removed. The base model was not changed.
        </p>
      )}
      {job.state === 'cancelled' && (
        <p className="rounded-md bg-raised p-3 text-sm text-ink-muted">Cancelled. Temporary files were removed.</p>
      )}

      {!terminal && (
        <div className="space-y-1">
          <div className="flex justify-between text-xs text-ink-muted">
            <span>{p.detail || stateLabel(job.state)}</span>
            {p.iters ? (
              <span className="tabular-nums">
                step {p.iter ?? 0} of {p.iters}
              </span>
            ) : null}
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-raised" role="progressbar" aria-valuenow={jobPercent(job)} aria-valuemin={0} aria-valuemax={100}>
            <div
              className="h-full bg-primary transition-[width] duration-500"
              style={{ width: `${downloading ? Math.round(((p.download_bytes ?? 0) / (p.download_total ?? 1)) * 100) : jobPercent(job)}%` }}
            />
          </div>
          {downloading && (
            <p className="text-xs text-ink-faint">
              Downloaded {formatBytes(p.download_bytes)} of {formatBytes(p.download_total)}
            </p>
          )}
        </div>
      )}

      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-4">
        <Metric label="Elapsed" value={formatDuration(elapsedSec(job, now))} />
        <Metric label="Remaining" value={p.remaining_sec != null && !terminal ? `about ${formatDuration(p.remaining_sec)}` : '—'} />
        <Metric label="Epoch" value={p.epoch != null ? `${p.epoch.toFixed(1)} of ${p.epochs ?? job.hyper.epochs}` : '—'} />
        <Metric label="Speed" value={p.tokens_per_sec ? `${Math.round(p.tokens_per_sec)} tokens/s` : '—'} />
        <Metric label="Training loss" value={p.train_loss != null ? p.train_loss.toFixed(3) : '—'} />
        <Metric label="Validation loss" value={p.val_loss != null ? p.val_loss.toFixed(3) : '—'} />
        <Metric label="Peak memory" value={p.peak_memory_gb ? `${p.peak_memory_gb.toFixed(1)} GB` : '—'} />
      </dl>
      <p className="text-xs text-ink-faint">
        Loss shows how closely the model reproduces your examples. A lower number does not guarantee better answers, so
        compare the results before you deploy.
      </p>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs text-ink-faint">{label}</dt>
      <dd className="tabular-nums text-ink">{value}</dd>
    </div>
  )
}

