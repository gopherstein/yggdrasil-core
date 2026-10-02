import { afterEach, describe, expect, it } from 'vitest'
import { applyLanguage } from '@/i18n'
import { categoryLabel } from '@/features/settings/notificationLabels'
import { statusOf } from '@/features/tools/mcpShared'
import type { MCPServer } from '@/types/api'
import { notificationLabel, scheduleLabel } from './parseRequest'

afterEach(async () => {
  await applyLanguage('en')
  localStorage.clear()
})

const pseudo = /^\[!! .* !!\]$/

describe('automation and tool text', () => {
  it('reads schedules the same way in English', () => {
    expect(scheduleLabel({ kind: 'daily', time_zone: 'UTC', hour: 8, minute: 0 })).toBe('Every day at 8:00 AM')
    expect(scheduleLabel({ kind: 'weekly', time_zone: 'UTC', hour: 18, minute: 30, weekday: 5 })).toBe('Every Friday at 6:30 PM')
    expect(scheduleLabel({ kind: 'interval', time_zone: 'UTC', every_seconds: 3 * 3600 })).toBe('Every 3 hours')
    expect(notificationLabel({ mode: 'condition', condition: { kind: 'threshold', op: 'below', value: 500 } })).toBe(
      'Notify when the price is below $500',
    )
  })

  it('shows schedules, notices, categories, and tool sources in the App language', async () => {
    await applyLanguage('en-XA')
    expect(scheduleLabel({ kind: 'weekly', time_zone: 'UTC', hour: 9, minute: 0, weekday: 1 })).toMatch(pseudo)
    expect(notificationLabel({ mode: 'change' })).toMatch(pseudo)
    expect(categoryLabel('automation')).toMatch(pseudo)
    const source = { status: 'ready', tools: [{}, {}], where: 'local', running: true } as unknown as MCPServer
    expect(statusOf(source).text).toMatch(pseudo)
  })
})
