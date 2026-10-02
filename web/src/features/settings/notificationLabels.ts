import i18n from '@/i18n'
import type { AppNotification, NotificationCategory } from '@/types/api'

/** The categories, in order; each is notifications:categories.<category> in the catalog. */
export const CATEGORIES: NotificationCategory[] = ['automation', 'approval', 'model', 'training', 'health', 'system']

/** A category's name in the App language. */
export function categoryLabel(category: NotificationCategory): string {
  return CATEGORIES.includes(category) ? i18n.t(`notifications:categories.${category}`) : category
}

/** What happened to a notification outside the app, when something needs saying. */
export function deliveryNote(n: AppNotification): string {
  const ds = n.deliveries ?? []
  if (ds.some((d) => d.status === 'failed')) return i18n.t('notifications:delivery.failed')
  if (ds.some((d) => d.status === 'held')) return i18n.t('notifications:delivery.held')
  if (ds.some((d) => d.status === 'pending' && d.attempts > 0)) return i18n.t('notifications:delivery.retrying')
  return ''
}
