import { useTranslation } from 'react-i18next'
import { EmptyState } from '@/components/ui/EmptyState'

export function OrchestratorsPage() {
  const { t } = useTranslation('profiles')
  return (
    <div className="w-full min-w-0 space-y-6">
      <header>
        <h1 className="font-display text-3xl font-semibold text-ink">{t('orchestrators.title')}</h1>
        <p className="mt-1 text-sm text-ink-muted">{t('orchestrators.description')}</p>
      </header>

      <EmptyState
        title={t('orchestrators.emptyTitle')}
        description={t('orchestrators.emptyDescription')}
      />
    </div>
  )
}
