import i18n from '@/i18n'
import type { AppNotification, LocalizedText, NotificationCategory } from '@/types/api'

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

function textOf(part: LocalizedText): string {
  if (!part.key) return part.text ?? ''
  return i18n.t(part.key, part.params ?? {})
}

/**
 * A notification's title and body in the App language. Core sends notices
 * as catalog keys with their values (multilingual spec §22); one without, such
 * as an automation result, shows its own text.
 */
export function noticeText(n: Pick<AppNotification, 'title' | 'body' | 'message'>): { title: string; body: string } {
  if (!n.message) return { title: n.title, body: n.body }
  // Chinese and Japanese don't put spaces between sentences.
  const sep = /^(zh|ja)\b/.test(i18n.language) ? '' : ' '
  const body = (n.message.body ?? []).map(textOf).map((s) => s.trim()).filter(Boolean).join(sep)
  return { title: textOf(n.message.title).trim() || n.title, body }
}
