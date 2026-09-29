import { useEffect, useState } from 'react'
import type { AIProfile, Automation, AutomationInput, AutomationSchedule, ToolRecord } from '@/types/api'
import {
  civilInputValue,
  civilToISO,
  localTimeZone,
  notificationLabel,
  parseAutomationRequest,
  scheduleLabel,
} from './parseRequest'

const WEEKDAYS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

interface AutomationFormProps {
  profiles: AIProfile[]
  tools: ToolRecord[]
  initial?: Automation | null
  seedDescription?: string
  pending: boolean
  error?: string
  onCancel: () => void
  onSubmit: (input: AutomationInput) => void
}

export function AutomationForm({ profiles, tools, initial, seedDescription = '', pending, error, onCancel, onSubmit }: AutomationFormProps) {
  const zone = initial?.schedule.time_zone || localTimeZone()
  const [description, setDescription] = useState('')
  const [notes, setNotes] = useState<string[]>([])
  const [parseError, setParseError] = useState('')
  const [name, setName] = useState(initial?.name ?? '')
  const [prompt, setPrompt] = useState(initial?.prompt ?? '')
  const [profileID, setProfileID] = useState(initial?.profile_id || profiles.find((p) => p.id === 'general-assistant')?.id || profiles[0]?.id || '')
  const [schedule, setSchedule] = useState<AutomationSchedule>(initial?.schedule ?? { kind: 'daily', time_zone: zone, hour: 8, minute: 0 })
  const [mode, setMode] = useState(initial?.notification.mode ?? 'always')
  const [conditionKind, setConditionKind] = useState(initial?.notification.condition?.kind ?? 'threshold')
  const [op, setOp] = useState(initial?.notification.condition?.op ?? 'below')
  const [value, setValue] = useState(String(initial?.notification.condition?.value ?? ''))
  const [selectedTools, setSelectedTools] = useState<string[]>(initial?.tools ?? [])

  useEffect(() => {
    if (!seedDescription) return
    setDescription(seedDescription)
    try {
      const parsed = parseAutomationRequest(seedDescription, new Date(), zone)
      setName(parsed.name)
      setPrompt(parsed.prompt)
      setSchedule(parsed.schedule)
      setMode(parsed.notification.mode)
      setConditionKind(parsed.notification.condition?.kind ?? 'threshold')
      setOp(parsed.notification.condition?.op ?? 'below')
      setValue(parsed.notification.condition?.value == null ? '' : String(parsed.notification.condition.value))
      setNotes(parsed.notes)
      setParseError('')
    } catch (err) {
      setParseError(err instanceof Error ? err.message : 'That description could not be read.')
    }
  }, [seedDescription, zone])

  const readTools = tools.filter((tool) => tool.risk === 'read' && tool.enabled)

  function applyDescription() {
    try {
      const parsed = parseAutomationRequest(description, new Date(), schedule.time_zone || zone)
      setName(parsed.name)
      setPrompt(parsed.prompt)
      setSchedule(parsed.schedule)
      setMode(parsed.notification.mode)
      setConditionKind(parsed.notification.condition?.kind ?? 'threshold')
      setOp(parsed.notification.condition?.op ?? 'below')
      setValue(parsed.notification.condition?.value == null ? '' : String(parsed.notification.condition.value))
      setNotes(parsed.notes)
      setParseError('')
    } catch (err) {
      setParseError(err instanceof Error ? err.message : 'That description could not be read.')
    }
  }

  function submit() {
    const notification =
      mode === 'condition'
        ? {
            mode,
            condition:
              conditionKind === 'threshold'
                ? { kind: conditionKind, op, value: Number(value) }
                : { kind: conditionKind },
          }
        : { mode }
    onSubmit({
      name: name.trim(),
      prompt: prompt.trim(),
      profile_id: profileID,
      schedule,
      notification,
      tools: selectedTools,
    })
  }

  const preview: AutomationInput = {
    name,
    prompt,
    profile_id: profileID,
    schedule,
    notification: { mode },
  }

  return (
    <form
      className="card space-y-4"
      onSubmit={(event) => {
        event.preventDefault()
        submit()
      }}
    >
      <div>
        <h2 className="font-display text-lg font-semibold text-ink">{initial ? 'Edit automation' : 'New automation'}</h2>
        <p className="mt-1 text-sm text-ink-muted">
          Describe the task in ordinary language, then adjust the schedule before saving.
        </p>
      </div>
      <label className="block space-y-1 text-sm">
        <span className="text-ink-muted">Describe the task</span>
        <textarea
          className="field min-h-24 w-full"
          value={description}
          placeholder="Every morning at 8:00 AM, check this product and tell me if the price is below $500."
          onChange={(event) => setDescription(event.target.value)}
        />
      </label>
      <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={applyDescription}>
        Apply description
      </button>
      {parseError && <p className="text-sm text-danger">{parseError}</p>}
      {notes.map((note) => (
        <p key={note} className="text-sm text-ink-muted">
          {note}
        </p>
      ))}
      <label className="block space-y-1 text-sm">
        <span className="text-ink-muted">Name</span>
        <input className="field w-full" value={name} onChange={(event) => setName(event.target.value)} required />
      </label>
      <label className="block space-y-1 text-sm">
        <span className="text-ink-muted">Prompt</span>
        <textarea className="field min-h-28 w-full" value={prompt} onChange={(event) => setPrompt(event.target.value)} required />
      </label>
      <label className="block space-y-1 text-sm">
        <span className="text-ink-muted">Profile</span>
        <select className="field w-full" value={profileID} onChange={(event) => setProfileID(event.target.value)} required>
          {profiles.length === 0 && <option value="">No profiles yet</option>}
          {profiles.map((profile) => (
            <option key={profile.id} value={profile.id}>
              {profile.name}
            </option>
          ))}
        </select>
      </label>
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block space-y-1 text-sm">
          <span className="text-ink-muted">Schedule</span>
          <select
            className="field w-full"
            value={schedule.kind}
            onChange={(event) => setSchedule(changeKind(schedule, event.target.value as AutomationSchedule['kind']))}
          >
            <option value="daily">Every day</option>
            <option value="weekly">Every week</option>
            <option value="interval">On an interval</option>
            <option value="once">Once</option>
          </select>
        </label>
        <label className="block space-y-1 text-sm">
          <span className="text-ink-muted">Time zone</span>
          <input
            className="field w-full"
            value={schedule.time_zone}
            onChange={(event) => setSchedule({ ...schedule, time_zone: event.target.value })}
            required
          />
        </label>
      </div>
      <ScheduleFields schedule={schedule} onChange={setSchedule} />
      <p className="text-sm text-ink">{scheduleLabel(preview.schedule)} · {notificationLabel(previewNotification(mode, conditionKind, op, value))}</p>
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block space-y-1 text-sm">
          <span className="text-ink-muted">Notification</span>
          <select className="field w-full" value={mode} onChange={(event) => setMode(event.target.value as AutomationInput['notification']['mode'])}>
            <option value="always">Notify every time</option>
            <option value="condition">Notify on a condition</option>
            <option value="change">Notify when the result changes</option>
            <option value="none">Store the result only</option>
          </select>
        </label>
        {mode === 'condition' && (
          <label className="block space-y-1 text-sm">
            <span className="text-ink-muted">Condition</span>
            <select
              className="field w-full"
              value={conditionKind}
              onChange={(event) => setConditionKind(event.target.value as 'threshold' | 'available' | 'significant')}
            >
              <option value="threshold">Price</option>
              <option value="available">Becomes available</option>
              <option value="significant">Significant result</option>
            </select>
          </label>
        )}
      </div>
      {mode === 'condition' && conditionKind === 'threshold' && (
        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block space-y-1 text-sm">
            <span className="text-ink-muted">Price is</span>
            <select className="field w-full" value={op} onChange={(event) => setOp(event.target.value as 'below' | 'above')}>
              <option value="below">below</option>
              <option value="above">above</option>
            </select>
          </label>
          <label className="block space-y-1 text-sm">
            <span className="text-ink-muted">Amount</span>
            <input className="field w-full" inputMode="decimal" value={value} onChange={(event) => setValue(event.target.value)} required />
          </label>
        </div>
      )}
      <fieldset className="space-y-2">
        <legend className="text-sm text-ink-muted">Read-only tools</legend>
        <p className="text-xs text-ink-faint">
          Leave these empty to use every read-only tool the profile already allows. Write tools stay off for scheduled runs.
        </p>
        <div className="grid gap-2 sm:grid-cols-2">
          {readTools.map((tool) => (
            <label key={tool.id} className="flex items-start gap-2 text-sm text-ink">
              <input
                type="checkbox"
                className="mt-1"
                checked={selectedTools.includes(tool.id)}
                onChange={(event) => {
                  setSelectedTools((current) =>
                    event.target.checked ? [...current, tool.id] : current.filter((id) => id !== tool.id),
                  )
                }}
              />
              <span>
                {tool.name}
                <span className="block text-xs text-ink-faint">{tool.description}</span>
              </span>
            </label>
          ))}
        </div>
      </fieldset>
      {error && <p className="text-sm text-danger">{error}</p>}
      <div className="flex gap-2">
        <button type="submit" className="btn-primary px-3 py-1.5 text-xs" disabled={pending || profiles.length === 0}>
          {pending ? 'Saving…' : 'Save'}
        </button>
        <button type="button" className="btn-secondary px-3 py-1.5 text-xs" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </form>
  )
}

function ScheduleFields({
  schedule,
  onChange,
}: {
  schedule: AutomationSchedule
  onChange: (schedule: AutomationSchedule) => void
}) {
  if (schedule.kind === 'interval') {
    const { amount, unit } = splitInterval(schedule.every_seconds ?? 6 * 3600)
    return (
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="block space-y-1 text-sm">
          <span className="text-ink-muted">Every</span>
          <input
            className="field w-full"
            type="number"
            min={1}
            value={amount}
            onChange={(event) => onChange({ ...schedule, every_seconds: Math.max(1, Number(event.target.value) || 1) * unitSeconds(unit) })}
          />
        </label>
        <label className="block space-y-1 text-sm">
          <span className="text-ink-muted">Unit</span>
          <select
            className="field w-full"
            value={unit}
            onChange={(event) => {
              const next = event.target.value as IntervalUnit
              onChange({ ...schedule, every_seconds: amount * unitSeconds(next) })
            }}
          >
            <option value="minutes">minutes</option>
            <option value="hours">hours</option>
            <option value="days">days</option>
          </select>
        </label>
      </div>
    )
  }
  if (schedule.kind === 'once') {
    return (
      <label className="block space-y-1 text-sm">
        <span className="text-ink-muted">When</span>
        <input
          className="field w-full"
          type="datetime-local"
          value={civilInputValue(schedule.at, schedule.time_zone)}
          onChange={(event) => onChange({ ...schedule, at: event.target.value ? civilToISO(event.target.value, schedule.time_zone) : undefined })}
          required
        />
      </label>
    )
  }
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <label className="block space-y-1 text-sm">
        <span className="text-ink-muted">Time</span>
        <input
          className="field w-full"
          type="time"
          value={`${String(schedule.hour ?? 0).padStart(2, '0')}:${String(schedule.minute ?? 0).padStart(2, '0')}`}
          onChange={(event) => {
            const [hour, minute] = event.target.value.split(':').map(Number)
            onChange({ ...schedule, hour, minute })
          }}
          required
        />
      </label>
      {schedule.kind === 'weekly' && (
        <label className="block space-y-1 text-sm">
          <span className="text-ink-muted">Day</span>
          <select
            className="field w-full"
            value={schedule.weekday ?? 1}
            onChange={(event) => onChange({ ...schedule, weekday: Number(event.target.value) })}
          >
            {WEEKDAYS.map((day, index) => (
              <option key={day} value={index}>
                {day}
              </option>
            ))}
          </select>
        </label>
      )}
    </div>
  )
}

type IntervalUnit = 'minutes' | 'hours' | 'days'

function unitSeconds(unit: IntervalUnit): number {
  if (unit === 'days') return 86400
  if (unit === 'hours') return 3600
  return 60
}

function splitInterval(seconds: number): { amount: number; unit: IntervalUnit } {
  if (seconds % 86400 === 0 && seconds >= 86400) return { amount: seconds / 86400, unit: 'days' }
  if (seconds % 3600 === 0 && seconds >= 3600) return { amount: seconds / 3600, unit: 'hours' }
  return { amount: Math.max(1, Math.round(seconds / 60)), unit: 'minutes' }
}

function changeKind(schedule: AutomationSchedule, kind: AutomationSchedule['kind']): AutomationSchedule {
  if (kind === 'once') return { kind, time_zone: schedule.time_zone, at: schedule.at }
  if (kind === 'interval') return { kind, time_zone: schedule.time_zone, every_seconds: schedule.every_seconds || 6 * 3600 }
  if (kind === 'weekly') {
    return { kind, time_zone: schedule.time_zone, hour: schedule.hour ?? 8, minute: schedule.minute ?? 0, weekday: schedule.weekday ?? 1 }
  }
  return { kind: 'daily', time_zone: schedule.time_zone, hour: schedule.hour ?? 8, minute: schedule.minute ?? 0 }
}

function previewNotification(mode: AutomationInput['notification']['mode'], kind: string, op: string, value: string): AutomationInput['notification'] {
  if (mode !== 'condition') return { mode }
  if (kind === 'threshold') {
    return { mode, condition: { kind: 'threshold', op: op === 'above' ? 'above' : 'below', value: Number(value) || 0 } }
  }
  if (kind === 'available') return { mode, condition: { kind: 'available' } }
  return { mode, condition: { kind: 'significant' } }
}
