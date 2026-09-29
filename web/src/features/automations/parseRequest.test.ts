import { describe, expect, it } from 'vitest'
import { civilToISO, parseAutomationRequest } from './parseRequest'

const zone = 'America/Los_Angeles'
const morning = new Date('2026-09-28T15:00:00Z')

describe('parseAutomationRequest', () => {
  it('turns a morning price check into a daily threshold task', () => {
    const parsed = parseAutomationRequest(
      'Every morning at 8:00 AM, check this product and tell me if the price is below $500.',
      morning,
      zone,
    )
    expect(parsed.name).toBe('Price below $500')
    expect(parsed.schedule).toMatchObject({ kind: 'daily', time_zone: zone, hour: 8, minute: 0 })
    expect(parsed.notification).toEqual({
      mode: 'condition',
      condition: { kind: 'threshold', op: 'below', value: 500 },
    })
    expect(parsed.prompt).toContain('{"price": 420}')
    expect(parsed.notes).toEqual([])
  })

  it('turns a stock check into an interval that notifies when available', () => {
    const parsed = parseAutomationRequest(
      'Every six hours, check whether this item is back in stock. Notify me only when it becomes available.',
      morning,
      zone,
    )
    expect(parsed.name).toBe('Stock check')
    expect(parsed.schedule).toMatchObject({ kind: 'interval', every_seconds: 6 * 3600 })
    expect(parsed.notification.mode).toBe('condition')
    expect(parsed.notification.condition).toEqual({ kind: 'available' })
    expect(parsed.prompt).toContain('{"available": true}')
  })

  it('turns a Friday release check into a weekly task and notes the default time', () => {
    const parsed = parseAutomationRequest(
      'Every Friday, check for new releases of this software and summarize what changed.',
      morning,
      zone,
    )
    expect(parsed.name).toBe('Release check')
    expect(parsed.schedule).toMatchObject({ kind: 'weekly', weekday: 5, hour: 8, minute: 0 })
    expect(parsed.notification.mode).toBe('always')
    expect(parsed.prompt).not.toContain('{"price"')
    expect(parsed.notes).toEqual(['No time was given, so this runs at 8:00 AM.'])
  })

  it('schedules a one-time research prompt for tomorrow morning in the task time zone', () => {
    const parsed = parseAutomationRequest('Run this research prompt once tomorrow at 9:00 AM.', morning, zone)
    expect(parsed.name).toBe('Research')
    expect(parsed.schedule.kind).toBe('once')
    expect(parsed.schedule.at).toBe('2026-09-29T16:00:00.000Z')
    expect(parsed.notification.mode).toBe('always')
  })

  it('keeps store-only and change requests out of the price condition', () => {
    const quiet = parseAutomationRequest("Don't notify me. Every day at 7:00 AM, check the news.", morning, zone)
    expect(quiet.notification.mode).toBe('none')
    expect(quiet.schedule).toMatchObject({ kind: 'daily', hour: 7, minute: 0 })

    const change = parseAutomationRequest('Every day at 7:00 AM, notify me only when the page changes.', morning, zone)
    expect(change.notification.mode).toBe('change')
  })

  it('converts a civil time in Pacific time to the matching UTC instant', () => {
    expect(civilToISO('2026-01-15T09:00', zone)).toBe('2026-01-15T17:00:00.000Z')
  })
})
