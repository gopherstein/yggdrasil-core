import i18n from '@/i18n'
import type { AutomationCondition, AutomationNotification, AutomationSchedule } from '@/types/api'

// The parser reads English requests ("every morning at 8"); what it shows the
// person, such as schedules, notes, and errors, is in the App language.

export interface ParsedAutomation {
  name: string
  prompt: string
  schedule: AutomationSchedule
  notification: AutomationNotification
  notes: string[]
}

const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'] as const

/** A weekday's name in the App language: 0 is Sunday. */
export function weekdayName(index: number): string {
  return i18n.t(`automations:weekdays.${index}`)
}

const PRICE_INSTRUCTION =
  'Include a JSON object in the result with the numeric price, for example {"price": 420}.'
const AVAILABLE_INSTRUCTION =
  'Include a JSON object in the result, {"available": true} when the item is available and {"available": false} when it is not.'
const SIGNIFICANT_INSTRUCTION =
  'Include a JSON object in the result, {"significant": true} when this is worth a notification and {"significant": false} when it is not.'

const QUANTITIES: Record<string, number> = {
  a: 1,
  an: 1,
  one: 1,
  two: 2,
  three: 3,
  four: 4,
  five: 5,
  six: 6,
  seven: 7,
  eight: 8,
  nine: 9,
  ten: 10,
  eleven: 11,
  twelve: 12,
  fifteen: 15,
  twenty: 20,
  thirty: 30,
  forty: 40,
  fortyfive: 45,
  fifty: 50,
  sixty: 60,
  ninety: 90,
}

export function localTimeZone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

export function parseAutomationRequest(text: string, now: Date, timeZone: string): ParsedAutomation {
  const original = text.trim()
  if (!original) {
    throw new Error(i18n.t('automations:parse.describe'))
  }
  if (!timeZone) {
    throw new Error(i18n.t('automations:parse.timeZone'))
  }
  const normalized = original.toLowerCase().replace(/\s+/g, ' ')
  const schedule = parseSchedule(normalized, now, timeZone)
  const notification = parseNotification(normalized)
  return {
    name: automationName(original, notification),
    prompt: withSignalInstruction(readableTask(original, notification), notification),
    schedule: schedule.schedule,
    notification,
    notes: schedule.notes,
  }
}

export function scheduleLabel(schedule: AutomationSchedule): string {
  switch (schedule.kind) {
    case 'once':
      return schedule.at
        ? i18n.t('automations:schedule.onceAt', { when: formatWhen(schedule.at, schedule.time_zone) })
        : i18n.t('automations:schedule.once')
    case 'daily':
      return i18n.t('automations:schedule.daily', { time: clockLabel(schedule.hour ?? 0, schedule.minute ?? 0) })
    case 'weekly':
      return i18n.t('automations:schedule.weekly', {
        day: weekdayName(schedule.weekday ?? 0),
        time: clockLabel(schedule.hour ?? 0, schedule.minute ?? 0),
      })
    case 'interval':
      return intervalLabel(schedule.every_seconds ?? 0)
    default:
      return i18n.t('automations:schedule.scheduled')
  }
}

export function notificationLabel(notification: AutomationNotification): string {
  switch (notification.mode) {
    case 'none':
      return i18n.t('automations:notify.none')
    case 'change':
      return i18n.t('automations:notify.change')
    case 'condition':
      return conditionLabel(notification.condition)
    default:
      return i18n.t('automations:notify.always')
  }
}

export function formatWhen(iso: string | undefined, timeZone: string): string {
  if (!iso) return i18n.t('automations:time.notScheduled')
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return i18n.t('automations:time.notScheduled')
  return new Intl.DateTimeFormat(undefined, {
    timeZone: timeZone || 'UTC',
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

export function civilInputValue(iso: string | undefined, timeZone: string): string {
  if (!iso) return ''
  const parts = zonedParts(new Date(iso), timeZone)
  const month = String(parts.month).padStart(2, '0')
  const day = String(parts.day).padStart(2, '0')
  const hour = String(parts.hour).padStart(2, '0')
  const minute = String(parts.minute).padStart(2, '0')
  return `${parts.year}-${month}-${day}T${hour}:${minute}`
}

export function civilToISO(value: string, timeZone: string): string {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/)
  if (!match) {
    throw new Error(i18n.t('automations:parse.chooseDateTime'))
  }
  return instantInZone(
    Number(match[1]),
    Number(match[2]),
    Number(match[3]),
    Number(match[4]),
    Number(match[5]),
    timeZone,
  ).toISOString()
}

function parseSchedule(text: string, now: Date, timeZone: string): { schedule: AutomationSchedule; notes: string[] } {
  const notes: string[] = []
  const interval = parseInterval(text)
  if (interval) {
    return { schedule: { kind: 'interval', time_zone: timeZone, every_seconds: interval }, notes }
  }
  const weekday = parseWeekday(text)
  if (weekday != null) {
    const clock = parseClock(text)
    const hour = clock?.hour ?? 8
    const minute = clock?.minute ?? 0
    if (!clock) notes.push(i18n.t('automations:parse.noTime8'))
    return {
      schedule: { kind: 'weekly', time_zone: timeZone, hour, minute, weekday },
      notes,
    }
  }
  if (isDaily(text)) {
    const clock = parseClock(text)
    let hour = clock?.hour
    let minute = clock?.minute
    if (!clock) {
      const named = namedDaypart(text)
      hour = named.hour
      minute = named.minute
      notes.push(named.note)
    }
    return {
      schedule: { kind: 'daily', time_zone: timeZone, hour: hour ?? 8, minute: minute ?? 0 },
      notes,
    }
  }
  if (/\b(?:once|tomorrow|today)\b/.test(text)) {
    const clock = parseClock(text)
    const hour = clock?.hour ?? 9
    const minute = clock?.minute ?? 0
    if (!clock) notes.push(i18n.t('automations:parse.noTime9'))
    const today = zonedParts(now, timeZone)
    const day = /\btoday\b/.test(text) && !/\btomorrow\b/.test(text) ? today : addDays(today, 1)
    const at = instantInZone(day.year, day.month, day.day, hour, minute, timeZone)
    return { schedule: { kind: 'once', time_zone: timeZone, at: at.toISOString() }, notes }
  }
  throw new Error(i18n.t('automations:parse.describeWhen'))
}

function parseInterval(text: string): number | null {
  if (/\bevery half an? hour\b/.test(text)) return 30 * 60
  const match = text.match(/\bevery\s+([a-z0-9-]+)\s+(second|minute|hour|day)s?\b/)
  if (!match) return null
  const amount = quantity(match[1])
  if (amount == null) return null
  if (match[2] === 'day' && amount === 1) return null
  const unit = match[2] === 'second' ? 1 : match[2] === 'minute' ? 60 : match[2] === 'hour' ? 3600 : 86400
  return amount * unit
}

function quantity(token: string): number | null {
  if (/^\d+$/.test(token)) return Number(token)
  const word = token.replace(/-/g, '')
  if (word === 'other') return 2
  return QUANTITIES[word] ?? null
}

function parseWeekday(text: string): number | null {
  const match = text.match(/\b(?:every|each|on)\s+(sundays?|mondays?|tuesdays?|wednesdays?|thursdays?|fridays?|saturdays?)\b/)
  if (!match) return null
  const name = match[1].replace(/s$/, '')
  return WEEKDAYS.findIndex((day) => day.toLowerCase() === name)
}

function isDaily(text: string): boolean {
  return /\b(?:every|each)\s+(?:morning|evening|night|afternoon|day)\b/.test(text) || /\bdaily\b/.test(text) || /\bevery day\b/.test(text)
}

function namedDaypart(text: string): { hour: number; minute: number; note: string } {
  if (/\bevening\b/.test(text)) return { hour: 18, minute: 0, note: i18n.t('automations:parse.evening') }
  if (/\bnight\b/.test(text)) return { hour: 21, minute: 0, note: i18n.t('automations:parse.night') }
  if (/\bafternoon\b/.test(text)) return { hour: 15, minute: 0, note: i18n.t('automations:parse.afternoon') }
  return { hour: 8, minute: 0, note: i18n.t('automations:parse.morning') }
}

function parseClock(text: string): { hour: number; minute: number } | null {
  const withMeridiem = text.match(/\b(?:at\s+)?(\d{1,2})(?::(\d{2}))?\s*(a\.?m\.?|p\.?m\.?)\b/)
  if (withMeridiem) {
    return clockFrom(Number(withMeridiem[1]), Number(withMeridiem[2] ?? '0'), withMeridiem[3].startsWith('p'))
  }
  const twentyFour = text.match(/\bat\s+(\d{1,2}):(\d{2})\b/)
  if (!twentyFour) return null
  const hour = Number(twentyFour[1])
  const minute = Number(twentyFour[2])
  if (hour > 23 || minute > 59) return null
  return { hour, minute }
}

function clockFrom(hour: number, minute: number, pm: boolean): { hour: number; minute: number } {
  let next = hour % 12
  if (pm) next += 12
  if (minute > 59 || hour > 12 || hour < 1) {
    throw new Error(i18n.t('automations:parse.badTime'))
  }
  return { hour: next, minute }
}

function parseNotification(text: string): AutomationNotification {
  if (/\b(?:do not|don't) notify\b/.test(text) || /\bstore only\b/.test(text) || /\bno notification\b/.test(text)) {
    return { mode: 'none' }
  }
  if (
    /\b(?:only when|when|if) (?:it|the result|this|the content|the page) changes\b/.test(text) ||
    /\bnotify(?: me)? (?:only )?on change\b/.test(text)
  ) {
    return { mode: 'change' }
  }
  if (/\b(?:only if|only when|when|if)\b.{0,40}\bsignificant\b/.test(text) || /\bworth a notification\b/.test(text)) {
    return { mode: 'condition', condition: { kind: 'significant' } }
  }
  const threshold = text.match(
    /\b(below|under|less than|cheaper than|above|over|more than|greater than)\s+[$€£]?\s*(\d[\d,]*(?:\.\d+)?)/,
  )
  if (threshold) {
    const below = /below|under|less than|cheaper than/.test(threshold[1])
    return {
      mode: 'condition',
      condition: {
        kind: 'threshold',
        op: below ? 'below' : 'above',
        value: Number(threshold[2].replace(/,/g, '')),
      },
    }
  }
  if (
    /\b(?:back in stock|in stock|out of stock)\b/.test(text) ||
    /\b(?:becomes|become|gets) available\b/.test(text) ||
    /\bnotify(?: me)? (?:only )?when (?:it is |it's |it becomes )?available\b/.test(text)
  ) {
    return { mode: 'condition', condition: { kind: 'available' } }
  }
  return { mode: 'always' }
}

function readableTask(original: string, notification: AutomationNotification): string {
  let text = original.trim().replace(/[.]+$/, '')
  text = text.replace(/^(?:every|each)\s+[^,]+,\s+/i, '')
  text = text.replace(/\s+(?:and\s+)?tell me if the price is (?:below|under|above|over|less than|more than)\s+[$€£]?\s*\d[\d,]*(?:\.\d+)?$/i, '')
  text = text.replace(/\s+notify me only when it becomes available$/i, '')
  text = text.replace(/\s+/g, ' ').trim()
  if (!text) text = original.trim()
  if (notification.condition?.kind === 'threshold' && !/\bprice\b/i.test(text)) {
    text = `${text}. Report the current price`
  }
  const sentence = text.charAt(0).toUpperCase() + text.slice(1)
  return /[.!?]$/.test(sentence) ? sentence : `${sentence}.`
}

function automationName(original: string, notification: AutomationNotification): string {
  const condition = notification.condition
  if (notification.mode === 'condition' && condition?.kind === 'threshold') {
    const amount = formatAmount(condition.value ?? 0)
    return i18n.t(condition.op === 'above' ? 'automations:names.priceAbove' : 'automations:names.priceBelow', { amount })
  }
  if (condition?.kind === 'available') {
    return /\bstock\b/i.test(original) ? i18n.t('automations:names.stock') : i18n.t('automations:names.availability')
  }
  if (condition?.kind === 'significant') return i18n.t('automations:names.significance')
  if (/\breleases?\b/i.test(original)) return i18n.t('automations:names.release')
  if (/\bresearch\b/i.test(original)) return i18n.t('automations:names.research')
  const cleaned = original.replace(/\s+/g, ' ').trim()
  if (!cleaned) return i18n.t('automations:names.scheduled')
  const short = cleaned.length > 48 ? `${cleaned.slice(0, 48).trim()}…` : cleaned
  return short.charAt(0).toUpperCase() + short.slice(1)
}

// visibleTask removes the machine-readable result instruction from a stored prompt.
export function visibleTask(prompt: string): string {
  let text = prompt.trim()
  for (const line of [PRICE_INSTRUCTION, AVAILABLE_INSTRUCTION, SIGNIFICANT_INSTRUCTION]) {
    text = text.split(line).join('')
  }
  return text.replace(/\n{3,}/g, '\n\n').trim()
}

// composePrompt stores the task the user wrote and, when needed, the result instruction.
export function composePrompt(task: string, notification: AutomationNotification): string {
  return withSignalInstruction(visibleTask(task), notification)
}

export function resultProse(result: string | undefined): string {
  if (!result) return ''
  const withoutInstruction = visibleTask(result)
  return withoutInstruction
    .replace(/```json[\s\S]*?```/g, '')
    .replace(/\{[^{}]*"(?:price|available|significant)"[^{}]*\}/g, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

function withSignalInstruction(text: string, notification: AutomationNotification): string {
  const instruction = signalInstruction(notification)
  if (!instruction || text.includes('{"price"') || text.includes('{"available"') || text.includes('{"significant"')) {
    return text.trim()
  }
  return `${text.trim()}\n\n${instruction}`
}

function signalInstruction(notification: AutomationNotification): string {
  switch (notification.condition?.kind) {
    case 'threshold':
      return PRICE_INSTRUCTION
    case 'available':
      return AVAILABLE_INSTRUCTION
    case 'significant':
      return SIGNIFICANT_INSTRUCTION
    default:
      return ''
  }
}

function formatAmount(value: number): string {
  return Number.isInteger(value) ? `$${value}` : `$${value.toFixed(2)}`
}

function conditionLabel(condition: AutomationCondition | undefined): string {
  if (!condition) return i18n.t('automations:notify.condition')
  if (condition.kind === 'threshold') {
    return i18n.t(condition.op === 'above' ? 'automations:notify.priceAbove' : 'automations:notify.priceBelow', {
      amount: formatAmount(condition.value ?? 0),
    })
  }
  if (condition.kind === 'available') return i18n.t('automations:notify.available')
  return i18n.t('automations:notify.significant')
}

function intervalLabel(seconds: number): string {
  if (seconds <= 0) return i18n.t('automations:schedule.onInterval')
  if (seconds % 86400 === 0) {
    const days = seconds / 86400
    return days === 1 ? i18n.t('automations:schedule.everyDay') : i18n.t('automations:schedule.days', { count: days })
  }
  if (seconds % 3600 === 0) {
    const hours = seconds / 3600
    return hours === 1 ? i18n.t('automations:schedule.everyHour') : i18n.t('automations:schedule.hours', { count: hours })
  }
  if (seconds % 60 === 0) {
    const minutes = seconds / 60
    return minutes === 1 ? i18n.t('automations:schedule.everyMinute') : i18n.t('automations:schedule.minutes', { count: minutes })
  }
  return i18n.t('automations:schedule.seconds', { count: seconds })
}

function clockLabel(hour: number, minute: number): string {
  return i18n.t('automations:time.clock', {
    hour: hour % 12 === 0 ? 12 : hour % 12,
    minute: String(minute).padStart(2, '0'),
    meridiem: i18n.t(hour >= 12 ? 'automations:time.pm' : 'automations:time.am'),
  })
}

interface CivilParts {
  year: number
  month: number
  day: number
  hour: number
  minute: number
}

function zonedParts(date: Date, timeZone: string): CivilParts {
  const fmt = new Intl.DateTimeFormat('en-US', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  })
  const parts = Object.fromEntries(fmt.formatToParts(date).map((part) => [part.type, part.value]))
  return {
    year: Number(parts.year),
    month: Number(parts.month),
    day: Number(parts.day),
    hour: Number(parts.hour) % 24,
    minute: Number(parts.minute),
  }
}

function addDays(parts: CivilParts, days: number): CivilParts {
  const next = new Date(Date.UTC(parts.year, parts.month - 1, parts.day + days))
  return {
    year: next.getUTCFullYear(),
    month: next.getUTCMonth() + 1,
    day: next.getUTCDate(),
    hour: parts.hour,
    minute: parts.minute,
  }
}

function instantInZone(year: number, month: number, day: number, hour: number, minute: number, timeZone: string): Date {
  const utcGuess = Date.UTC(year, month - 1, day, hour, minute)
  const observed = zonedParts(new Date(utcGuess), timeZone)
  const observedUTC = Date.UTC(observed.year, observed.month - 1, observed.day, observed.hour, observed.minute)
  return new Date(utcGuess - (observedUTC - utcGuess))
}
