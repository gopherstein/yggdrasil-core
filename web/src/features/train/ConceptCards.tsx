import { Trans, useTranslation } from 'react-i18next'
import { trainingExplainer } from './display'

/** The two ideas the Train page teaches, side by side and visually distinct. */
export function ConceptCards({ compact = false }: { compact?: boolean }) {
  const { t } = useTranslation('train')
  const strong = { strong: <strong className="text-ink" /> }
  return (
    <div className="space-y-3">
      <div className="grid gap-3 md:grid-cols-2">
        <div className="card-outline space-y-2 border-l-4 !border-l-primary p-4">
          <p className="label-caps text-primary">{t('concepts.training')}</p>
          <h3 className="font-display text-lg font-semibold text-ink">{t('concepts.trainingTitle')}</h3>
          {!compact && <p className="text-sm text-ink-muted">{t('concepts.trainingBody')}</p>}
          <p className="text-xs text-ink-faint">{t('concepts.trainingUse')}</p>
        </div>
        <div className="card-outline space-y-2 border-l-4 !border-l-mimir p-4">
          <p className="label-caps text-mimir">{t('concepts.knowledge')}</p>
          <h3 className="font-display text-lg font-semibold text-ink">{t('concepts.knowledgeTitle')}</h3>
          {!compact && <p className="text-sm text-ink-muted">{t('concepts.knowledgeBody')}</p>}
          <p className="text-xs text-ink-faint">{t('concepts.knowledgeUse')}</p>
        </div>
      </div>
      <p className="text-sm text-ink-muted">{trainingExplainer()}</p>
      <details className="text-sm text-ink-muted">
        <summary className="cursor-pointer text-ink">{t('concepts.how')}</summary>
        <div className="mt-2 space-y-2">
          <p>
            <Trans t={t} i18nKey="concepts.lora" components={strong} />
          </p>
          <p>
            <Trans t={t} i18nKey="concepts.rag" components={strong} />
          </p>
          <p>
            <Trans t={t} i18nKey="concepts.epoch" components={strong} />
          </p>
        </div>
      </details>
    </div>
  )
}
