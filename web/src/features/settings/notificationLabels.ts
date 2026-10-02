import type { AppNotification, NotificationCategory } from '@/types/api'

export const CATEGORY_LABEL: Record<NotificationCategory, string> = {
  automation: 'Automations',
  approval: 'Approvals',
  model: 'Models',
  training: 'Training',
  health: 'Health',
  system: 'System',
}

/** What happened to a notification outside the app, when something needs saying. */
export function deliveryNote(n: AppNotification): string {
  const ds = n.deliveries ?? []
  if (ds.some((d) => d.status === 'failed')) return 'not delivered everywhere'
  if (ds.some((d) => d.status === 'held')) return 'held for quiet hours'
  if (ds.some((d) => d.status === 'pending' && d.attempts > 0)) return 'retrying delivery'
  return ''
}
