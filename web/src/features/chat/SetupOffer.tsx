import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { formatSize } from '@/i18n/format'
import { api, type MediaKind } from '@/lib/api'
import type { SetupOffer } from '@/types/api'

function gb(bytes: number): string {
  return formatSize(bytes, { base: 1000 })
}

/**
 * Offers to install what a request needed (Gungnir §29): what it is, its
 * size, and where it runs. Once it is installed from here, the request is
 * sent again so it is finished.
 */
export function SetupOfferCard({ offer, onContinue }: { offer: SetupOffer; onContinue: (text: string) => void }) {
  const { t } = useTranslation('chat')
  const queryClient = useQueryClient()
  const [started, setStarted] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const continued = useRef(false)
  // The setup each installable ability uses.
  const kind: MediaKind | null = offer.ability === 'image_generation' ? 'images' : offer.ability === 'video_generation' ? 'video' : null
  const statusQuery = useQuery({
    queryKey: [kind ?? 'none', 'setup'],
    queryFn: () => api.getMediaSetup(kind!),
    enabled: kind !== null,
    retry: false,
    refetchInterval: (query) => (query.state.data?.job?.running ? 1000 : false),
  })
  const status = statusQuery.data
  const job = status?.job
  const ready = !!status?.ready

  // Installed from this card: finish the request, once.
  useEffect(() => {
    if (!ready || !started || continued.current) return
    continued.current = true
    void queryClient.invalidateQueries({ queryKey: ['tools'] })
    if (offer.request) onContinue(offer.request)
  }, [ready, started, offer.request, onContinue, queryClient])

  if (!kind || !status) return null

  const where = offer.node_name ? `${offer.node_name} (${t('setup.thisComputer')})` : t('setup.thisComputer')
  const percent = job && job.total_bytes > 0 ? Math.round((job.done_bytes / job.total_bytes) * 100) : 0
  const failed = error ?? (job && !job.running ? job.error : undefined)

  return (
    <div className="mt-3 space-y-2 rounded-xl border border-line/70 bg-surface px-3 py-2.5 text-sm">
      {ready ? (
        <div className="flex flex-wrap items-center gap-2">
          <span>{started && offer.request ? t('setup.continuing') : t('setup.ready', { label: offer.label })}</span>
          {!started && offer.request ? (
            <button type="button" className="btn-secondary px-3 py-1 text-xs" onClick={() => onContinue(offer.request!)}>
              {t('setup.continue')}
            </button>
          ) : null}
        </div>
      ) : job?.running ? (
        <div className="space-y-1.5">
          <div className="flex items-center justify-between gap-2 text-xs text-ink-muted">
            <span>
              {job.stage === 'program'
                ? t('setup.installing')
                : t('setup.downloading', { done: gb(job.done_bytes), total: gb(job.total_bytes) })}
            </span>
            <button
              type="button"
              className="text-xs text-ink-faint underline-offset-2 hover:text-ink hover:underline"
              onClick={() => void api.cancelMediaSetup(kind).then(() => statusQuery.refetch())}
            >
              {t('setup.stop')}
            </button>
          </div>
          <div className="h-2 overflow-hidden rounded-full bg-raised" role="progressbar" aria-valuenow={percent} aria-valuemin={0} aria-valuemax={100}>
            <div className="h-full bg-primary transition-[width] duration-500" style={{ width: `${percent}%` }} />
          </div>
        </div>
      ) : (
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <span className="min-w-0 flex-1 text-xs text-ink-muted">
            {t('setup.details', { name: offer.name, size: gb(offer.size_bytes), node: where })}
          </span>
          <button
            type="button"
            className="btn-primary px-3 py-1.5 text-xs"
            onClick={async () => {
              setError(null)
              try {
                const next = await api.startMediaSetup(kind, offer.option)
                setStarted(true)
                if (next) queryClient.setQueryData([kind, 'setup'], next)
                else void statusQuery.refetch()
              } catch (err) {
                setError(err instanceof Error ? err.message : String(err))
              }
            }}
          >
            {t('setup.install')}
          </button>
        </div>
      )}
      {failed && !ready ? <p className="text-xs text-danger">{failed}</p> : null}
    </div>
  )
}
