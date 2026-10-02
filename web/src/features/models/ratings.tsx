import { useTranslation } from 'react-i18next'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { api, ApiError } from '@/lib/api'
import { RATING_TAGS, useCommunityRatings, useModelRating, useRatingDialog } from './ratingsState'
import { formatDecimal, formatNumber } from '@/i18n/format'
import type { ModelRating, RatingStats, RatingTag } from '@/types/api'

function Stars({ value }: { value: number }) {
  const full = Math.round(value)
  return (
    <span aria-hidden className="text-warning">
      {'★'.repeat(full)}
      <span className="text-ink-faint">{'★'.repeat(5 - full)}</span>
    </span>
  )
}

/** One line of community ratings: the score, how many, and a label when there are few. */
function StatsLine({ label, stats }: { label: string; stats: RatingStats }) {
  const { t } = useTranslation('models')
  return (
    <p className="text-xs text-ink-muted">
      <span className="font-medium text-ink tabular-nums">★ {formatDecimal(stats.weighted_score, 1)}</span> {label}
      <span className="text-ink-faint"> · </span>
      <span className="tabular-nums">{t('ratings.count', { count: stats.ratings, formatted: formatNumber(stats.ratings) })}</span>
      {stats.confidence !== 'community' ? (
        <span className="ms-1.5 status-chip bg-raised/80 text-ink-muted" title={t('ratings.limitedHint')}>
          {t(`ratings.confidence.${stats.confidence}`)}
        </span>
      ) : null}
    </p>
  )
}

/** Community ratings of a model, from hardware like this computer's and overall, and this person's own. */
export function CommunityScore({ modelId }: { modelId: string }) {
  const { t } = useTranslation('models')
  const community = useCommunityRatings().data
  const entry = community?.models?.[modelId]
  if (!entry?.similar && !entry?.overall) return null
  return (
    <div className="mt-2 space-y-0.5">
      {entry.similar ? <StatsLine label={t(`ratings.similar.${entry.similar.tier}`)} stats={entry.similar} /> : null}
      {entry.overall ? <StatsLine label={t('ratings.overall')} stats={entry.overall} /> : null}
    </div>
  )
}

/** The Rate button, showing this person's stars once they have rated. */
export function RateButton({ modelId, modelName }: { modelId: string; modelName: string }) {
  const { t } = useTranslation('models')
  const open = useRatingDialog((s) => s.open)
  const rating = useModelRating(modelId).data
  return (
    <button
      type="button"
      className="btn-secondary px-3 py-1.5 text-xs"
      aria-label={rating?.stars ? t('ratings.yourRatingLabel', { model: modelName, count: rating.stars }) : t('ratings.rateLabel', { model: modelName })}
      onClick={() => open(modelId, modelName)}
    >
      {rating?.stars ? <Stars value={rating.stars} /> : t('ratings.rate')}
    </button>
  )
}

/** The rating dialog, for whichever model useRatingDialog has open. */
export function RatingDialogHost() {
  const model = useRatingDialog((s) => s.model)
  const close = useRatingDialog((s) => s.close)
  if (!model) return null
  return <RateModelDialog key={model.id} modelId={model.id} modelName={model.name} onClose={close} />
}

function RateModelDialog({ modelId, modelName, onClose }: { modelId: string; modelName: string; onClose: () => void }) {
  const { t } = useTranslation('models')
  const queryClient = useQueryClient()
  const ratingQuery = useModelRating(modelId)
  const rating = ratingQuery.data
  const [stars, setStars] = useState(0)
  const [tags, setTags] = useState<RatingTag[]>([])
  const [share, setShare] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!rating) return
    setStars(rating.stars ?? 0)
    setTags(rating.tags ?? [])
    setShare(rating.shared)
  }, [rating])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  const refresh = (next?: ModelRating) => {
    if (next) queryClient.setQueryData(['model-rating', modelId], next)
    else void queryClient.invalidateQueries({ queryKey: ['model-rating', modelId] })
  }
  const failed = (err: unknown) => {
    setError(err instanceof Error ? err.message : String(err))
    // A rating the service could not take is still saved here.
    if (err instanceof ApiError && err.status === 502) refresh()
  }
  const save = useMutation({
    mutationFn: () => api.putModelRating(modelId, { stars, tags, share: share && Boolean(rating?.rateable) }),
    onSuccess: (next) => {
      refresh(next ?? undefined)
      onClose()
    },
    onError: failed,
  })
  const remove = useMutation({
    mutationFn: () => api.deleteModelRating(modelId),
    onSuccess: () => {
      refresh()
      onClose()
    },
    onError: failed,
  })
  const toggleTag = (tag: RatingTag) => setTags((list) => (list.includes(tag) ? list.filter((x) => x !== tag) : [...list, tag]))
  const shares = rating?.shares

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-ink/40 p-4 sm:items-center" role="presentation" onClick={onClose}>
      <div
        className="card max-h-[90vh] w-full max-w-lg space-y-4 overflow-y-auto shadow-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="rate-model-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div>
          <h2 id="rate-model-title" className="font-display text-lg font-semibold text-ink">
            {t('ratings.title', { model: modelName })}
          </h2>
          <p className="mt-1 text-sm text-ink-muted">{t('ratings.intro')}</p>
        </div>

        <div role="radiogroup" aria-label={t('ratings.starsLabel')} className="flex gap-1">
          {[1, 2, 3, 4, 5].map((n) => (
            <button
              key={n}
              type="button"
              role="radio"
              aria-checked={stars === n}
              aria-label={t('ratings.stars', { count: n })}
              className={['text-3xl leading-none transition', n <= stars ? 'text-warning' : 'text-ink-faint hover:text-ink-muted'].join(' ')}
              onClick={() => setStars(n)}
            >
              ★
            </button>
          ))}
        </div>

        <fieldset>
          <legend className="text-sm font-medium text-ink">{t('ratings.tagsLabel')}</legend>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {RATING_TAGS.map((tag) => (
              <button
                key={tag}
                type="button"
                aria-pressed={tags.includes(tag)}
                className={[
                  'status-chip transition',
                  tags.includes(tag) ? 'bg-primary/20 text-ink ring-1 ring-primary' : 'bg-raised/80 text-ink-muted hover:text-ink',
                ].join(' ')}
                onClick={() => toggleTag(tag)}
              >
                {t(`ratings.tags.${tag}`)}
              </button>
            ))}
          </div>
        </fieldset>

        {rating && !rating.rateable ? (
          <p className="text-xs text-ink-muted">{t('ratings.notShareable', { reason: rating.reason ?? '' })}</p>
        ) : shares ? (
          <div className="rounded-lg border border-line/60 p-3">
            <label className="flex items-start gap-2 text-sm text-ink">
              <input type="checkbox" className="mt-1" checked={share} onChange={(e) => setShare(e.target.checked)} />
              <span>
                {t('ratings.share')}
                <span className="block text-xs text-ink-muted">{t('ratings.shareHint')}</span>
              </span>
            </label>
            <details className="mt-2 text-xs text-ink-muted" open={share}>
              <summary className="cursor-pointer">{t('ratings.whatIsShared')}</summary>
              <dl className="mt-2 space-y-0.5">
                <SharedRow label={t('ratings.sent.destination')} value={shares.destination} />
                <SharedRow label={t('ratings.sent.rating')} value={t('ratings.sent.ratingValue')} />
                <SharedRow
                  label={t('ratings.sent.model')}
                  value={`${shares.model.id} · ${shares.model.quantization} · ${shares.model.format} · ${shares.model.runtime} (${shares.model.backend})`}
                />
                <SharedRow
                  label={t('ratings.sent.hardware')}
                  value={`${shares.hardware.vendor} ${shares.hardware.family} · ${t('ratings.sent.memory', {
                    type: shares.hardware.memory_type,
                    gb: shares.hardware.memory_bucket_gb,
                  })} · ${shares.hardware.platform} ${shares.hardware.architecture}`}
                />
                <SharedRow label={t('ratings.sent.id')} value={t('ratings.sent.idValue')} />
              </dl>
              <p className="mt-2">{t('ratings.sent.never')}</p>
            </details>
          </div>
        ) : null}

        {error ? (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        ) : null}

        <div className="flex flex-wrap justify-between gap-2">
          <div>
            {rating?.stars ? (
              <button type="button" className="btn-secondary text-danger" disabled={remove.isPending} onClick={() => remove.mutate()}>
                {t('ratings.remove')}
              </button>
            ) : null}
          </div>
          <div className="flex gap-2">
            <button type="button" className="btn-secondary" onClick={onClose}>
              {t('ratings.cancel')}
            </button>
            <button type="button" className="btn-primary" disabled={stars === 0 || save.isPending} onClick={() => save.mutate()}>
              {share && rating?.rateable ? t('ratings.saveAndShare') : t('ratings.save')}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}

function SharedRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex min-w-0 justify-between gap-3">
      <dt className="shrink-0">{label}</dt>
      <dd className="min-w-0 break-anywhere text-end text-ink">{value}</dd>
    </div>
  )
}

/** Asks for a rating of the model that answered, once it has been used a while. */
export function RatingPrompt({ modelId, modelName }: { modelId: string; modelName: string }) {
  const { t } = useTranslation('models')
  const queryClient = useQueryClient()
  const open = useRatingDialog((s) => s.open)
  const [hidden, setHidden] = useState(false)
  const rating = useModelRating(modelId).data
  if (hidden || !rating?.ask) return null
  const dismiss = async () => {
    setHidden(true)
    try {
      await api.dismissModelRating(modelId)
    } finally {
      void queryClient.invalidateQueries({ queryKey: ['model-rating', modelId] })
    }
  }
  return (
    <div className="mx-auto mb-2 flex w-full max-w-3xl flex-wrap items-center justify-between gap-2 rounded-lg border border-line/60 bg-raised/40 px-3 py-2 text-sm">
      <span className="text-ink">{t('ratings.prompt', { model: modelName })}</span>
      <span className="flex gap-2">
        <button
          type="button"
          className="btn-primary px-3 py-1 text-xs"
          onClick={() => {
            setHidden(true)
            open(modelId, modelName)
          }}
        >
          {t('ratings.rate')}
        </button>
        <button type="button" className="btn-secondary px-3 py-1 text-xs" onClick={() => setHidden(true)}>
          {t('ratings.notNow')}
        </button>
        <button type="button" className="px-2 py-1 text-xs text-ink-muted hover:text-ink" onClick={() => void dismiss()}>
          {t('ratings.dontAsk')}
        </button>
      </span>
    </div>
  )
}
