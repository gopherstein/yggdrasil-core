import i18n from '@/i18n'

/** How a role reads in run details: the planner, each worker, the answer, and the reviewer (spec O7). */
export function roleLabel(role?: string): string {
  if (!role || role === 'assistant') return i18n.t('chat:roles.answer')
  const slot = /^worker:(\d+)$/.exec(role)
  if (slot) return i18n.t('chat:roles.worker', { n: slot[1] })
  if (role === 'planner') return i18n.t('chat:roles.planner')
  if (role === 'reviewer') return i18n.t('chat:roles.reviewer')
  return role.charAt(0).toUpperCase() + role.slice(1)
}

/** Planner first, then the workers in order, the answer, and the reviewer. */
export function roleOrder(role?: string): number {
  if (role === 'planner') return 0
  const slot = /^worker:(\d+)$/.exec(role ?? '')
  if (slot) return 1 + Number(slot[1]) / 1000
  if (role === 'reviewer') return 3
  return 2
}
