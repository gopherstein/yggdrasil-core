import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '@/lib/api'
import type { DiskEncryption } from '@/types/api'

/** The at-rest point, from whether this computer's disk is encrypted (#213). */
function atRest(t: (key: string, options?: Record<string, unknown>) => string, disk?: DiskEncryption): { text: string; warn: boolean } {
  const method = disk?.method
  switch (disk?.state) {
    case 'on':
      return { text: t('yourData.points.atRestOn', { method }), warn: false }
    case 'encrypting':
      return { text: t('yourData.points.atRestEncrypting', { method }), warn: false }
    case 'off':
      if (method) return { text: t(`yourData.points.atRestOff${method}`), warn: true }
  }
  return { text: t('yourData.points.atRest'), warn: false }
}

/**
 * Where the person's chats and data live, and what is and isn't encrypted,
 * stated plainly so nobody has to guess (see docs/privacy.md). Whether this
 * computer's disk is encrypted is checked, not assumed.
 */
export function YourData() {
  const { t } = useTranslation('settings')
  // The same query as What left this computer, so it's asked once.
  const overview = useQuery({ queryKey: ['privacy'], queryFn: () => api.getPrivacy(), retry: false })
  const rest = atRest(t as unknown as (key: string, options?: Record<string, unknown>) => string, overview.data?.disk_encryption)
  return (
    <section className="card space-y-3" aria-labelledby="your-data-title">
      <h2 id="your-data-title" className="section-title">
        {t('yourData.title')}
      </h2>
      <ul className="list-disc space-y-1.5 ps-5 text-sm text-ink">
        <li>{t('yourData.points.stored')}</li>
        <li>{t('yourData.points.leaves')}</li>
        <li className={rest.warn ? 'text-warning' : undefined}>{rest.text}</li>
        <li>{t('yourData.points.inTransit')}</li>
      </ul>
    </section>
  )
}
