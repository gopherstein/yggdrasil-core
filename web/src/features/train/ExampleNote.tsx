import { useTranslation } from 'react-i18next'
import type { SpecializedAIView } from '@/types/api'
import type { StepID } from './display'


/** The example AI's note for a step: what to look at here. Each is train:example.<step>. */
export function ExampleNote({ view, step }: { view: SpecializedAIView; step: StepID }) {
  const { t } = useTranslation('train')
  if (!view.example) return null
  return (
    <div className="rounded-lg border border-info/30 bg-info/10 p-3 text-sm text-ink">
      <p className="label-caps mb-1 text-info">{t('example.title')}</p>
      <p className="text-ink-muted">{t(`example.${step}`)}</p>
    </div>
  )
}
