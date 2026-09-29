import type { AutomationCondition, AutomationNotification, AutomationSchedule } from '@/types/api'

export interface ParsedAutomation {
  name: string
  prompt: string
  schedule: AutomationSchedule
  notification: AutomationNotification
  notes: string[]
}

const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'] as const

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
    throw new Error('Describe the automation.')
  }
  if (!timeZone) {
    throw new Error('A time zone is required.')
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
      return schedule.at ? `Once at ${formatWhen(schedule.at, schedule.time_zone)}` : 'Once'
    case 'daily':
      return `Every day at ${clockLabel(schedule.hour ?? 0, schedule.minute ?? 0)}`
    case 'weekly':
      return `Every ${WEEKDAYS[schedule.weekday ?? 0]} at ${clockLabel(schedule.hour ?? 0, schedule.minute ?? 0)}`
    case 'interval':
      return intervalLabel(schedule.every_seconds ?? 0)
    default:
      return 'Scheduled'
  }
}

export function notificationLabel(notification: AutomationNotification): string {
  switch (notification.mode) {
    case 'none':
      return 'Store the result only'
    case 'change':
      return 'Notify when the result changes'
    case 'condition':
      return conditionLabel(notification.condition)
    default:
      return 'Notify every time'
  }
}

export function formatWhen(iso: string | undefined, timeZone: string): string {
  if (!iso) return 'Not scheduled'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return 'Not scheduled'
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
    throw new Error('Choose a date and time.')
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
    if (!clock) notes.push('No time was given, so this runs at 8:00 AM.')
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
    if (!clock) notes.push('No time was given, so this runs at 9:00 AM.')
    const today = zonedParts(now, timeZone)
    const day = /\btoday\b/.test(text) && !/\btomorrow\b/.test(text) ? today : addDays(today, 1)
    const at = instantInZone(day.year, day.month, day.day, hour, minute, timeZone)
    return { schedule: { kind: 'once', time_zone: timeZone, at: at.toISOString() }, notes }
  }
  throw new Error('Describe when it should run, for example “every morning at 8:00 AM”.')
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
  if (/\bevening\b/.test(text)) return { hour: 18, minute: 0, note: 'Evening means 6:00 PM unless you set another time.' }
  if (/\bnight\b/.test(text)) return { hour: 21, minute: 0, note: 'Night means 9:00 PM unless you set another time.' }
  if (/\bafternoon\b/.test(text)) return { hour: 15, minute: 0, note: 'Afternoon means 3:00 PM unless you set another time.' }
  return { hour: 8, minute: 0, note: 'Morning means 8:00 AM unless you set another time.' }
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
    throw new Error('That time is not valid.')
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
    return condition.op === 'above' ? `Price above ${amount}` : `Price below ${amount}`
  }
  if (condition?.kind === 'available') {
    return /\bstock\b/i.test(original) ? 'Stock check' : 'Availability check'
  }
  if (condition?.kind === 'significant') return 'Significance check'
  if (/\breleases?\b/i.test(original)) return 'Release check'
  if (/\bresearch\b/i.test(original)) return 'Research'
  const cleaned = original.replace(/\s+/g, ' ').trim()
  if (!cleaned) return 'Scheduled task'
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
  if (!condition) return 'Notify on a condition'
  if (condition.kind === 'threshold') {
    return `Notify when the price is ${condition.op === 'above' ? 'above' : 'below'} ${formatAmount(condition.value ?? 0)}`
  }
  if (condition.kind === 'available') return 'Notify when it becomes available'
  return 'Notify when the result is significant'
}

function intervalLabel(seconds: number): string {
  if (seconds <= 0) return 'On an interval'
  if (seconds % 86400 === 0) {
    const days = seconds / 86400
    return days === 1 ? 'Every day' : `Every ${days} days`
  }
  if (seconds % 3600 === 0) {
    const hours = seconds / 3600
    return hours === 1 ? 'Every hour' : `Every ${hours} hours`
  }
  if (seconds % 60 === 0) {
    const minutes = seconds / 60
    return minutes === 1 ? 'Every minute' : `Every ${minutes} minutes`
  }
  return `Every ${seconds} seconds`
}

function clockLabel(hour: number, minute: number): string {
  const meridiem = hour >= 12 ? 'PM' : 'AM'
  const shown = hour % 12 === 0 ? 12 : hour % 12
  return `${shown}:${String(minute).padStart(2, '0')} ${meridiem}`
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
