import { useTranslation } from 'react-i18next'

// The points, in order; each is settings:yourData.points.<key>.
const POINTS = ['stored', 'leaves', 'atRest', 'inTransit'] as const

/**
 * Where the person's chats and data live, and what is and isn't encrypted,
 * stated plainly so nobody has to guess (see docs/privacy.md).
 */
export function YourData() {
  const { t } = useTranslation('settings')
  return (
    <section className="card space-y-3" aria-labelledby="your-data-title">
      <h2 id="your-data-title" className="section-title">
        {t('yourData.title')}
      </h2>
      <ul className="list-disc space-y-1.5 ps-5 text-sm text-ink">
        {POINTS.map((key) => (
          <li key={key}>{t(`yourData.points.${key}`)}</li>
        ))}
      </ul>
    </section>
  )
}
