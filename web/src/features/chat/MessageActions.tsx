import { useTranslation } from 'react-i18next'
import type { Message } from '@/types/api'

/**
 * Retry and edit (#447): a retry or an edit makes a new version of its point
 * in the chat, and the earlier ones stay. These are the controls under a
 * message: switching versions, trying an answer again, and editing a sent
 * message.
 */

// Shown on hover or focus where there's a mouse, always on touch.
const REVEAL = 'opacity-0 transition-opacity focus-within:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100'

/** ‹ 1 / 2 ›, when a point in the chat has more than one version. */
export function VersionSwitch({
  message,
  disabled,
  onShow,
}: {
  message: Message
  disabled?: boolean
  onShow: (id: string) => void
}) {
  const { t } = useTranslation('chat')
  const v = message.versions
  if (!v || v.count < 2) return null
  const at = v.index - 1
  return (
    <span className="inline-flex items-center gap-0.5 text-xs text-ink-muted" role="group" aria-label={t('versions.label')}>
      <button
        type="button"
        className="rounded px-1 hover:text-ink disabled:opacity-40"
        disabled={disabled || at <= 0}
        aria-label={t('versions.previous')}
        onClick={() => onShow(v.ids[at - 1])}
      >
        ‹
      </button>
      <span aria-live="polite">{t('versions.position', { index: v.index, count: v.count })}</span>
      <button
        type="button"
        className="rounded px-1 hover:text-ink disabled:opacity-40"
        disabled={disabled || at >= v.count - 1}
        aria-label={t('versions.next')}
        onClick={() => onShow(v.ids[at + 1])}
      >
        ›
      </button>
    </span>
  )
}

/** Try again, or try again with another model, under an answer. */
export function AnswerActions({
  disabled,
  models,
  onRetry,
}: {
  disabled?: boolean
  /** The models to offer, with Auto first. */
  models: { id: string; name: string }[]
  onRetry: (modelId?: string) => void
}) {
  const { t } = useTranslation('chat')
  return (
    <span className={`inline-flex items-center gap-1 ${REVEAL}`}>
      <button type="button" className="rounded px-1.5 py-0.5 text-xs text-ink-muted hover:bg-ink/5 hover:text-ink" disabled={disabled} onClick={() => onRetry()}>
        {t('versions.tryAgain')}
      </button>
      {models.length > 1 ? (
        <select
          className="rounded bg-transparent py-0.5 text-xs text-ink-muted hover:text-ink"
          aria-label={t('versions.tryAgainWith')}
          value=""
          disabled={disabled}
          onChange={(e) => e.target.value && onRetry(e.target.value)}
        >
          <option value="">{t('versions.withAnother')}</option>
          {models.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name}
            </option>
          ))}
        </select>
      ) : null}
    </span>
  )
}

/** Edit, under a message the person sent. */
export function EditAction({ disabled, onEdit }: { disabled?: boolean; onEdit: () => void }) {
  const { t } = useTranslation('chat')
  return (
    <button
      type="button"
      className={`rounded px-1.5 py-0.5 text-xs text-ink-muted hover:bg-ink/5 hover:text-ink ${REVEAL}`}
      disabled={disabled}
      onClick={onEdit}
    >
      {t('versions.edit')}
    </button>
  )
}
