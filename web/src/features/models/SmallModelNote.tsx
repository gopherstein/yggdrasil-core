import { useTranslation } from 'react-i18next'
import type { Model } from '@/types/api'
import { isSmallModel } from './modelPresentation'

/**
 * For a small model: a plain note that it can get details wrong, and a larger
 * model that fits this computer. Larger models get nothing, so the note stays
 * meaningful.
 */
export function SmallModelNote({
  model,
  alternative,
  onInstallAlternative,
}: {
  model: Model
  alternative?: Model | null
  onInstallAlternative?: (id: string) => void
}) {
  const { t } = useTranslation('models')
  if (!isSmallModel(model) || model.support_role) return null
  const other = alternative && alternative.id !== model.id ? alternative : null
  return (
    <div role="note" className="mt-2 rounded-lg bg-warning/10 px-2.5 py-2 text-xs leading-relaxed text-ink">
      <p>{t('small.note')}</p>
      {other ? (
        other.installed ? (
          <p className="mt-1 text-ink-muted">
            {t('small.installedOther', { model: other.display_name })}
          </p>
        ) : onInstallAlternative ? (
          <p className="mt-1 text-ink-muted">
            {t('small.fitsOther', { model: other.display_name })}{' '}
            <button
              type="button"
              className="font-medium text-primary underline-offset-2 hover:underline"
              onClick={() => onInstallAlternative(other.id)}
            >
              {t('small.installOther', { model: other.display_name })}
            </button>
          </p>
        ) : null
      ) : null}
    </div>
  )
}
