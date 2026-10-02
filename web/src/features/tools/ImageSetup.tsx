import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { formatSize } from '@/i18n/format'
import { api } from '@/lib/api'
import type { ImageSetup } from '@/types/api'

function gb(bytes: number): string {
  return formatSize(bytes, { base: 1000 })
}

/**
 * Image generation (Gungnir §17): set it up with a recommended model, watch
 * the download, and remove models to free space.
 */
export function ImageSetupCard() {
  const { t } = useTranslation('tools')
  const queryClient = useQueryClient()
  const [choice, setChoice] = useState<string | null>(null)
  const setupQuery = useQuery({
    queryKey: ['images', 'setup'],
    queryFn: () => api.getImageSetup(),
    retry: false,
    // Follow a setup while it runs.
    refetchInterval: (query) => (query.state.data?.job?.running ? 1000 : false),
  })
  const update = (status: ImageSetup | null) => {
    if (status) queryClient.setQueryData(['images', 'setup'], status)
    else void queryClient.invalidateQueries({ queryKey: ['images', 'setup'] })
    // Tools become ready, or stop being ready.
    void queryClient.invalidateQueries({ queryKey: ['tools'] })
  }
  const start = useMutation({ mutationFn: (id: string) => api.startImageSetup(id), onSuccess: update })
  const cancel = useMutation({ mutationFn: () => api.cancelImageSetup(), onSuccess: update })
  const remove = useMutation({ mutationFn: (id: string) => api.removeImageModel(id), onSuccess: update })

  // When a setup ends, the image tools become ready (or stay not ready).
  const wasRunning = useRef(false)
  const runningNow = !!setupQuery.data?.job?.running
  useEffect(() => {
    if (wasRunning.current && !runningNow) void queryClient.invalidateQueries({ queryKey: ['tools'] })
    wasRunning.current = runningNow
  }, [runningNow, queryClient])

  const status = setupQuery.data
  if (!status) return null
  const job = status.job
  const running = !!job?.running
  const active = status.models.find((m) => m.id === status.active)
  const selected = choice ?? (status.models.find((m) => m.recommended) ?? status.models[0])?.id
  const failed = start.error ?? remove.error
  const error = failed instanceof Error ? failed.message : job && !job.running ? job.error : undefined
  const percent = job && job.total_bytes > 0 ? Math.round((job.done_bytes / job.total_bytes) * 100) : 0

  return (
    <section className="card space-y-3" aria-labelledby="image-setup-title">
      <div>
        <h2 id="image-setup-title" className="section-title">
          {t('images.title')}
        </h2>
        <p className="mt-1 max-w-2xl text-sm text-ink-muted">
          {!status.supported
            ? t('images.unsupported', { reason: status.unsupported })
            : status.ready && active
              ? t('images.ready', { model: active.name })
              : t('images.notSetUp')}
        </p>
      </div>

      {status.supported && running && job ? (
        <div className="space-y-1.5">
          <div className="flex items-center justify-between text-xs text-ink-muted">
            <span>{job.stage === 'program' ? t('images.installingProgram') : t('images.downloadingModel')}</span>
            <span className="tabular-nums">{t('images.progress', { done: gb(job.done_bytes), total: gb(job.total_bytes) })}</span>
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-raised" role="progressbar" aria-valuenow={percent} aria-valuemin={0} aria-valuemax={100}>
            <div className="h-full bg-primary transition-[width] duration-500" style={{ width: `${percent}%` }} />
          </div>
          <button type="button" className="btn-secondary px-3 py-1.5 text-xs" disabled={cancel.isPending} onClick={() => cancel.mutate()}>
            {t('images.stop')}
          </button>
        </div>
      ) : null}

      {status.supported && !running ? (
        <ul className="space-y-2">
          {status.models.map((model) => (
            <li key={model.id} className="flex flex-wrap items-start gap-3 rounded-lg border border-line/70 p-3">
              {!model.installed ? (
                <input
                  type="radio"
                  name="image-model"
                  className="mt-1"
                  checked={selected === model.id}
                  onChange={() => setChoice(model.id)}
                  aria-label={model.name}
                />
              ) : null}
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-ink">
                  {model.name}
                  {model.recommended ? <span className="ml-2 text-xs font-normal text-success">{t('images.recommended')}</span> : null}
                  {model.id === status.active ? <span className="ml-2 text-xs font-normal text-ink-faint">{t('images.inUse')}</span> : null}
                </p>
                <p className="mt-0.5 text-xs text-ink-muted">{model.description}</p>
                <p className="mt-0.5 text-xs text-ink-faint">
                  {gb(model.size_bytes)} · {model.license}
                  {model.edits ? t('images.makesAndEdits') : t('images.makes')}
                </p>
              </div>
              {model.installed ? (
                <span className="flex gap-2">
                  {model.id !== status.active ? (
                    <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={() => start.mutate(model.id)}>
                      {t('images.use')}
                    </button>
                  ) : null}
                  <button
                    type="button"
                    className="btn-secondary px-3 py-1.5 text-xs"
                    disabled={remove.isPending}
                    onClick={() => remove.mutate(model.id)}
                  >
                    {t('images.remove')}
                  </button>
                </span>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}

      {status.supported && !running && selected && !status.models.find((m) => m.id === selected)?.installed ? (
        <button type="button" className="btn-primary px-4 py-2 text-sm" disabled={start.isPending} onClick={() => start.mutate(selected)}>
          {status.ready ? t('images.download') : t('images.setUp')}
        </button>
      ) : null}

      {error ? <p className="text-xs text-danger">{error}</p> : null}
    </section>
  )
}
